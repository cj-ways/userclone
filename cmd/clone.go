package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cj-ways/userclone/internal/config"
	"github.com/cj-ways/userclone/internal/git"
	gh "github.com/cj-ways/userclone/internal/github"
	gl "github.com/cj-ways/userclone/internal/gitlab"
	"github.com/spf13/cobra"
)

type repoEntry struct {
	name     string
	cloneURL string
}

type repoGroup struct {
	label   string
	dest    string
	entries []repoEntry
}

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone all repos from your account",
	Long:  `Clone your personal repos and optionally all organization/group repos.`,
	RunE:  runClone,
}

var (
	flagToken        string
	flagDest         string
	flagWithOrgs     bool
	flagOrg          []string
	flagOnlyOrgs     bool
	flagExclude      string
	flagSkipArchived bool
	flagSkipForks    bool
	flagPrivateOnly  bool
	flagPublicOnly   bool
	flagPick         bool
	flagDryRun       bool
	flagFlat         bool
	flagGitLab       bool
	flagGitLabURL    string
)

func init() {
	rootCmd.AddCommand(cloneCmd)

	cloneCmd.Flags().StringVarP(&flagToken, "token", "t", "", "GitHub/GitLab personal access token")
	cloneCmd.Flags().StringVarP(&flagDest, "dest", "d", "", "Base destination directory (default ~/Desktop)")
	cloneCmd.Flags().BoolVar(&flagWithOrgs, "with-orgs", false, "Also clone repos from all orgs you belong to")
	cloneCmd.Flags().StringArrayVarP(&flagOrg, "org", "o", nil, "Clone repos from specific org(s) only (repeatable)")
	cloneCmd.Flags().BoolVar(&flagOnlyOrgs, "only-orgs", false, "Skip personal repos, only clone org repos")
	cloneCmd.Flags().StringVarP(&flagExclude, "exclude", "e", "", "Comma-separated repo names to skip")
	cloneCmd.Flags().BoolVar(&flagSkipArchived, "skip-archived", false, "Skip archived repos")
	cloneCmd.Flags().BoolVar(&flagSkipForks, "skip-forks", false, "Skip forked repos")
	cloneCmd.Flags().BoolVar(&flagPrivateOnly, "private-only", false, "Only clone private repos")
	cloneCmd.Flags().BoolVar(&flagPublicOnly, "public-only", false, "Only clone public repos")
	cloneCmd.Flags().BoolVar(&flagPick, "pick", false, "Interactive checkbox selection per group")
	cloneCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview repos without cloning")
	cloneCmd.Flags().BoolVar(&flagFlat, "flat", false, "Don't group by org — clone everything into dest root")
	cloneCmd.Flags().BoolVar(&flagGitLab, "gitlab", false, "Use GitLab instead of GitHub")
	cloneCmd.Flags().StringVar(&flagGitLabURL, "gitlab-url", "https://gitlab.com", "Self-hosted GitLab URL")
}

func runClone(cmd *cobra.Command, args []string) error {
	if flagPrivateOnly && flagPublicOnly {
		return fmt.Errorf("--private-only and --public-only cannot be used together")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		cfg = &config.Config{DefaultDest: "~/Desktop", DefaultPlatform: "github"}
	}

	isGitLab := flagGitLab
	if !cmd.Flags().Changed("gitlab") && cfg.DefaultPlatform == "gitlab" {
		isGitLab = true
	}

	token := flagToken
	if token == "" {
		token = cfg.GetToken(isGitLab)
	}
	if token == "" {
		envName := "GITHUB_TOKEN"
		if isGitLab {
			envName = "GITLAB_TOKEN"
		}
		return fmt.Errorf("token is required. Use --token, %s env var, or set it in ~/.userclone.yml", envName)
	}

	dest := flagDest
	if dest == "" {
		dest = cfg.DefaultDest
	}
	if dest == "" {
		dest = "~/Desktop"
	}
	dest = expandHome(dest)

	withOrgs := flagWithOrgs
	if !cmd.Flags().Changed("with-orgs") {
		withOrgs = cfg.WithOrgs
	}

	excludeList := parseExclude(flagExclude)
	for _, e := range cfg.Exclude {
		excludeList = append(excludeList, e)
	}

	var groups []repoGroup

	if isGitLab {
		groups, err = fetchGitLab(token, cfg, dest, withOrgs, excludeList)
	} else {
		groups, err = fetchGitHub(token, cfg, dest, withOrgs, excludeList)
	}
	if err != nil {
		return err
	}

	if flagPick {
		groups, err = interactivePick(groups)
		if err != nil {
			return err
		}
	}

	if flagFlat {
		if err := checkFlatCollisions(groups); err != nil {
			return err
		}
	}

	return executeClone(groups, token, flagDryRun)
}

func authenticatedCloneURL(cloneURL, token string) string {
	if token != "" && strings.HasPrefix(cloneURL, "https://") {
		return strings.Replace(cloneURL, "https://", "https://oauth2:"+token+"@", 1)
	}
	return cloneURL
}

func fetchGitHub(token string, cfg *config.Config, dest string, withOrgs bool, excludeList []string) ([]repoGroup, error) {
	platform := "github"
	fmt.Printf("Fetching user profile from %s...\n", platform)

	user, err := gh.GetAuthenticatedUser(token)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Printf("Authenticated as: %s\n\n", user.Login)

	var groups []repoGroup

	if !flagOnlyOrgs {
		repos, err := gh.GetUserRepos(token, user.Login)
		if err != nil {
			return nil, err
		}

		var entries []repoEntry
		for _, r := range repos {
			if shouldExcludeGitHub(r, excludeList, cfg, "") {
				continue
			}
			entries = append(entries, repoEntry{name: r.Name, cloneURL: r.CloneURL})
		}

		if len(entries) > 0 {
			groups = append(groups, repoGroup{
				label:   fmt.Sprintf("Personal Repos (%d repos)", len(entries)),
				dest:    dest,
				entries: entries,
			})
		}
	}

	orgNames := flagOrg
	if len(orgNames) == 0 && (withOrgs || flagOnlyOrgs) {
		orgs, err := gh.GetUserOrgs(token)
		if err != nil {
			return nil, err
		}
		for _, o := range orgs {
			orgNames = append(orgNames, o.Login)
		}
	}

	for _, orgName := range orgNames {
		repos, err := gh.GetOrgRepos(token, orgName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch repos for org %s: %v\n", orgName, err)
			continue
		}

		var entries []repoEntry
		for _, r := range repos {
			if shouldExcludeGitHub(r, excludeList, cfg, orgName) {
				continue
			}
			entries = append(entries, repoEntry{name: r.Name, cloneURL: r.CloneURL})
		}

		if len(entries) > 0 {
			orgDest := filepath.Join(dest, orgName)
			if flagFlat {
				orgDest = dest
			}
			groups = append(groups, repoGroup{
				label:   fmt.Sprintf("Org: %s (%d repos)", orgName, len(entries)),
				dest:    orgDest,
				entries: entries,
			})
		}
	}

	return groups, nil
}

func fetchGitLab(token string, cfg *config.Config, dest string, withOrgs bool, excludeList []string) ([]repoGroup, error) {
	platform := "gitlab"
	gitlabURL := flagGitLabURL
	if gitlabURL == "" || gitlabURL == "https://gitlab.com" {
		if cfg.GitLab.URL != "" {
			gitlabURL = cfg.GitLab.URL
		} else {
			gitlabURL = "https://gitlab.com"
		}
	}
	gitlabURL = strings.TrimRight(gitlabURL, "/")

	fmt.Printf("Fetching user profile from %s...\n", platform)

	user, err := gl.GetAuthenticatedUser(token, gitlabURL)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Printf("Authenticated as: %s\n\n", user.Username)

	var groups []repoGroup

	if !flagOnlyOrgs {
		projects, err := gl.GetUserProjects(token, gitlabURL)
		if err != nil {
			return nil, err
		}

		var entries []repoEntry
		for _, p := range projects {
			if p.Namespace.Kind != "user" {
				continue
			}
			if shouldExcludeGitLab(p, excludeList, cfg, "") {
				continue
			}
			entries = append(entries, repoEntry{name: p.Name, cloneURL: p.HTTPURLToRepo})
		}

		if len(entries) > 0 {
			groups = append(groups, repoGroup{
				label:   fmt.Sprintf("Personal Repos (%d repos)", len(entries)),
				dest:    dest,
				entries: entries,
			})
		}
	}

	if withOrgs || flagOnlyOrgs || len(flagOrg) > 0 {
		glGroups, err := gl.GetUserGroups(token, gitlabURL)
		if err != nil {
			return nil, err
		}

		targetGroups := glGroups
		if len(flagOrg) > 0 {
			targetGroups = nil
			orgSet := make(map[string]bool)
			for _, o := range flagOrg {
				orgSet[strings.ToLower(o)] = true
			}
			found := make(map[string]bool)
			for _, g := range glGroups {
				pathLower := strings.ToLower(g.Path)
				nameLower := strings.ToLower(g.Name)
				if orgSet[pathLower] || orgSet[nameLower] {
					targetGroups = append(targetGroups, g)
					found[pathLower] = true
					found[nameLower] = true
				}
			}
			for _, o := range flagOrg {
				if !found[strings.ToLower(o)] {
					fmt.Fprintf(os.Stderr, "Warning: group %q not found in your GitLab groups\n", o)
				}
			}
		}

		for _, group := range targetGroups {
			projects, err := gl.GetGroupProjects(token, gitlabURL, group.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not fetch projects for group %s: %v\n", group.Path, err)
				continue
			}

			var entries []repoEntry
			for _, p := range projects {
				if shouldExcludeGitLab(p, excludeList, cfg, group.Path) {
					continue
				}
				entries = append(entries, repoEntry{name: p.Name, cloneURL: p.HTTPURLToRepo})
			}

			if len(entries) > 0 {
				groupDest := filepath.Join(dest, group.Path)
				if flagFlat {
					groupDest = dest
				}
				groups = append(groups, repoGroup{
					label:   fmt.Sprintf("Group: %s (%d repos)", group.Path, len(entries)),
					dest:    groupDest,
					entries: entries,
				})
			}
		}
	}

	return groups, nil
}

func shouldExcludeGitHub(r gh.Repo, excludeList []string, cfg *config.Config, orgName string) bool {
	for _, e := range excludeList {
		if strings.EqualFold(r.Name, strings.TrimSpace(e)) {
			return true
		}
	}
	if cfg.IsExcluded(r.Name, orgName) {
		return true
	}
	if flagSkipArchived && r.Archived {
		return true
	}
	if flagSkipForks && r.Fork {
		return true
	}
	if flagPrivateOnly && !r.Private {
		return true
	}
	if flagPublicOnly && r.Private {
		return true
	}
	return false
}

func shouldExcludeGitLab(p gl.Project, excludeList []string, cfg *config.Config, groupPath string) bool {
	for _, e := range excludeList {
		if strings.EqualFold(p.Name, strings.TrimSpace(e)) {
			return true
		}
	}
	if cfg.IsExcluded(p.Name, groupPath) {
		return true
	}
	if flagSkipArchived && p.Archived {
		return true
	}
	if flagSkipForks && p.IsFork() {
		return true
	}
	if flagPrivateOnly && !p.IsPrivate() {
		return true
	}
	if flagPublicOnly && !p.IsPublic() {
		return true
	}
	return false
}

func interactivePick(groups []repoGroup) ([]repoGroup, error) {
	var result []repoGroup

	for _, g := range groups {
		if len(g.entries) == 0 {
			continue
		}

		var names []string
		for _, e := range g.entries {
			names = append(names, e.name)
		}

		var selected []string
		prompt := &survey.MultiSelect{
			Message: fmt.Sprintf("Select repos from %s:", g.label),
			Options: names,
			Default: names,
		}

		if err := survey.AskOne(prompt, &selected); err != nil {
			return nil, err
		}

		selectedSet := make(map[string]bool)
		for _, s := range selected {
			selectedSet[s] = true
		}

		var filtered []repoEntry
		for _, e := range g.entries {
			if selectedSet[e.name] {
				filtered = append(filtered, e)
			}
		}

		if len(filtered) > 0 {
			result = append(result, repoGroup{
				label:   g.label,
				dest:    g.dest,
				entries: filtered,
			})
		}
	}

	return result, nil
}

func executeClone(groups []repoGroup, token string, dryRun bool) error {
	totalCloned, totalUpdated, totalUpToDate, totalSkipped, totalFailed := 0, 0, 0, 0, 0

	for _, g := range groups {
		fmt.Printf("━━ %s ━━━━━━━━━━━━━━━━━━━\n", g.label)
		fmt.Printf("Destination: %s\n\n", g.dest)

		for _, entry := range g.entries {
			if dryRun {
				if git.Exists(g.dest, entry.name) {
					fmt.Printf("  ~ %-40s exists (would update)\n", entry.name)
					totalUpdated++
				} else {
					fmt.Printf("  ~ %-40s would clone\n", entry.name)
					totalCloned++
				}
				continue
			}

			result := git.CloneOrPull(authenticatedCloneURL(entry.cloneURL, token), g.dest, entry.name)

			symbol := "+"
			switch result.Status {
			case "cloned":
				symbol = "+"
				totalCloned++
			case "updated":
				symbol = "^"
				totalUpdated++
			case "up to date":
				symbol = "-"
				totalUpToDate++
			case "skipped":
				symbol = "~"
				totalSkipped++
			case "failed":
				symbol = "!"
				totalFailed++
			}

			status := result.Status
			if result.Error != nil {
				if result.Status == "skipped" {
					status = fmt.Sprintf("skipped: %v", result.Error)
				} else {
					status = fmt.Sprintf("failed: %v", result.Error)
				}
			}

			fmt.Printf("  %s %-40s %s\n", symbol, entry.name, status)
		}

		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Dry run complete.  %d would clone  %d already exist\n", totalCloned, totalUpdated)
	} else {
		fmt.Printf("Done.  %d cloned  %d updated  %d up-to-date  %d skipped  %d failed\n",
			totalCloned, totalUpdated, totalUpToDate, totalSkipped, totalFailed)
	}

	if totalFailed > 0 {
		return fmt.Errorf("%d repos failed", totalFailed)
	}
	return nil
}

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func checkFlatCollisions(groups []repoGroup) error {
	seen := make(map[string]string) // repo name -> source group label
	var collisions []string

	for _, g := range groups {
		for _, e := range g.entries {
			if prev, ok := seen[e.name]; ok {
				collisions = append(collisions, fmt.Sprintf("  %q appears in both %q and %q", e.name, prev, g.label))
			} else {
				seen[e.name] = g.label
			}
		}
	}

	if len(collisions) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: --flat mode has name collisions:\n")
		for _, c := range collisions {
			fmt.Fprintln(os.Stderr, c)
		}
		fmt.Fprintln(os.Stderr, "The first occurrence will be cloned; duplicates will attempt to update it.")
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func parseExclude(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
