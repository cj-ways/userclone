package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cj-ways/userclone/internal/auth"
	"github.com/cj-ways/userclone/internal/config"
	gh "github.com/cj-ways/userclone/internal/github"
	gl "github.com/cj-ways/userclone/internal/gitlab"
	"github.com/spf13/cobra"
)

type wizardState struct {
	cmd *cobra.Command
	cfg *config.Config

	platform  string
	gitlabURL string

	token       string
	tokenSource string

	login       string
	displayName string

	basePath   string
	folderName string
	visibility string
	scope      string
	structure  string

	personalRepos []repoEntry
	orgGroups     []orgWithRepos

	groups []repoGroup
	dryRun bool

	legacyDest  bool // true when --dest flag used or non-interactive
	interactive bool
	prompted    bool
}

type orgWithRepos struct {
	name  string
	repos []repoEntry
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func runWizard(cmd *cobra.Command, cfg *config.Config) ([]repoGroup, string, bool, error) {
	w := &wizardState{
		cmd:         cmd,
		cfg:         cfg,
		dryRun:      flagDryRun,
		interactive: isInteractive(),
	}

	steps := []func() error{
		w.stepPlatform,
		w.stepAuth,
		w.stepPath,
		w.stepFolderName,
		w.stepVisibility,
		w.fetchRepos,
		w.stepScope,
		w.stepStructure,
		w.buildGroups,
		w.stepSummary,
	}

	for _, step := range steps {
		if err := step(); err != nil {
			return nil, "", false, err
		}
	}

	return w.groups, w.token, w.dryRun, nil
}

// --- Step 1: Platform ---

func (w *wizardState) stepPlatform() error {
	if w.cmd.Flags().Changed("gitlab") {
		if flagGitLab {
			w.platform = "gitlab"
		} else {
			w.platform = "github"
		}
		return nil
	}

	if !w.interactive {
		w.platform = w.cfg.DefaultPlatform
		if w.platform == "" {
			w.platform = "github"
		}
		return nil
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "Which platform?",
		Options: []string{"GitHub", "GitLab"},
		Default: "GitHub",
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}
	w.platform = strings.ToLower(choice)
	return nil
}

// --- Step 2: Auth ---

func (w *wizardState) stepAuth() error {
	// Flag
	if flagToken != "" {
		w.token = flagToken
		w.tokenSource = "flag"
		return w.validateToken()
	}

	// Config / env
	isGitLab := w.platform == "gitlab"
	if t := w.cfg.GetToken(isGitLab); t != "" {
		w.token = t
		w.tokenSource = "config"
		return w.validateToken()
	}

	if !w.interactive {
		envName := "GITHUB_TOKEN"
		if isGitLab {
			envName = "GITLAB_TOKEN"
		}
		return fmt.Errorf("token required. Use --token, %s env var, or set it in ~/.userclone.yml", envName)
	}

	// Interactive auth
	w.prompted = true
	var err error
	if w.platform == "github" {
		w.token, w.tokenSource, err = auth.GetGitHubToken()
	} else {
		w.token, w.tokenSource, err = auth.GetGitLabToken()
	}
	if err != nil {
		return err
	}

	if err := w.validateToken(); err != nil {
		return err
	}

	// Offer to save
	if w.tokenSource == "manual" || w.tokenSource == "device-flow" {
		if auth.OfferSaveToken() {
			if w.platform == "github" {
				w.cfg.GitHub.Token = w.token
			} else {
				w.cfg.GitLab.Token = w.token
			}
			if err := config.Save(w.cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save token: %v\n", err)
			} else {
				fmt.Println("  Token saved to ~/.userclone.yml")
			}
		}
	}

	return nil
}

func (w *wizardState) validateToken() error {
	if w.platform == "github" {
		user, err := gh.GetAuthenticatedUser(w.token)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		w.login = user.Login
		w.displayName = user.Name
		if w.displayName == "" {
			w.displayName = user.Login
		}
	} else {
		gitlabURL := flagGitLabURL
		if gitlabURL == "" || gitlabURL == "https://gitlab.com" {
			if w.cfg.GitLab.URL != "" {
				gitlabURL = w.cfg.GitLab.URL
			} else {
				gitlabURL = "https://gitlab.com"
			}
		}
		w.gitlabURL = strings.TrimRight(gitlabURL, "/")

		user, err := gl.GetAuthenticatedUser(w.token, w.gitlabURL)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		w.login = user.Username
		w.displayName = user.Name
		if w.displayName == "" {
			w.displayName = user.Username
		}
	}

	fmt.Printf("  Authenticated as %s (@%s)\n\n", w.displayName, w.login)
	return nil
}

// --- Step 3: Path ---

func (w *wizardState) stepPath() error {
	if w.cmd.Flags().Changed("dest") {
		w.basePath = expandHome(flagDest)
		w.legacyDest = true
		return nil
	}

	if !w.interactive {
		dest := w.cfg.DefaultDest
		if dest == "" {
			dest = "~/Desktop"
		}
		w.basePath = expandHome(dest)
		w.legacyDest = true
		return nil
	}

	defaultPath := w.cfg.DefaultDest
	if defaultPath == "" {
		defaultPath = "~/Desktop"
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "Where to clone?",
		Options: []string{
			fmt.Sprintf("Desktop (%s)", defaultPath),
			"Custom path",
		},
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	if strings.HasPrefix(choice, "Desktop") {
		w.basePath = expandHome(defaultPath)
	} else {
		var custom string
		input := &survey.Input{
			Message: "Enter path:",
			Default: defaultPath,
		}
		if err := survey.AskOne(input, &custom); err != nil {
			return err
		}
		w.basePath = expandHome(custom)
	}

	return nil
}

// --- Step 4: Folder Name ---

func (w *wizardState) stepFolderName() error {
	if w.legacyDest {
		return nil
	}

	options := []string{
		"GitHub User",
		w.displayName,
		"Custom",
	}
	// Deduplicate if display name happens to match
	if w.displayName == "GitHub User" {
		options = []string{"GitHub User", "Custom"}
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "Folder name for your repos?",
		Options: options,
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	if choice == "Custom" {
		var custom string
		input := &survey.Input{
			Message: "Enter folder name:",
		}
		if err := survey.AskOne(input, &custom); err != nil {
			return err
		}
		w.folderName = strings.TrimSpace(custom)
		if w.folderName == "" {
			w.folderName = "GitHub User"
		}
	} else {
		w.folderName = choice
	}

	return nil
}

// --- Step 5: Visibility ---

func (w *wizardState) stepVisibility() error {
	if w.cmd.Flags().Changed("private-only") {
		w.visibility = "private"
		return nil
	}
	if w.cmd.Flags().Changed("public-only") {
		w.visibility = "public"
		return nil
	}

	if !w.interactive {
		w.visibility = "all"
		return nil
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "Which repos?",
		Options: []string{"All repos", "Public only", "Private only"},
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	switch choice {
	case "Public only":
		w.visibility = "public"
	case "Private only":
		w.visibility = "private"
	default:
		w.visibility = "all"
	}
	return nil
}

// --- Step 6: Fetch Repos ---

func (w *wizardState) fetchRepos() error {
	fmt.Println("Fetching repos...")

	excludeList := parseExclude(flagExclude)
	excludeList = append(excludeList, w.cfg.Exclude...)

	// Determine whether to fetch orgs
	fetchOrgs := true
	if !w.interactive {
		// Non-interactive: only fetch orgs if explicitly requested
		if !w.cmd.Flags().Changed("with-orgs") && !w.cmd.Flags().Changed("only-orgs") && !w.cmd.Flags().Changed("org") {
			fetchOrgs = false
		} else {
			fetchOrgs = flagWithOrgs || flagOnlyOrgs || len(flagOrg) > 0
		}
	}

	skipPersonal := w.cmd.Flags().Changed("only-orgs") && flagOnlyOrgs

	if w.platform == "github" {
		return w.fetchGitHubRepos(excludeList, !skipPersonal, fetchOrgs)
	}
	return w.fetchGitLabRepos(excludeList, !skipPersonal, fetchOrgs)
}

func (w *wizardState) fetchGitHubRepos(excludeList []string, fetchPersonal, fetchOrgs bool) error {
	if fetchPersonal {
		repos, err := gh.GetUserRepos(w.token, w.login)
		if err != nil {
			return err
		}
		for _, r := range repos {
			if w.shouldExcludeGH(r, excludeList, "") {
				continue
			}
			w.personalRepos = append(w.personalRepos, repoEntry{name: r.Name, cloneURL: r.CloneURL})
		}
	}

	if fetchOrgs {
		specificOrgs := flagOrg
		if len(specificOrgs) > 0 {
			for _, orgName := range specificOrgs {
				w.fetchGitHubOrg(orgName, excludeList)
			}
		} else {
			orgs, err := gh.GetUserOrgs(w.token)
			if err != nil {
				return err
			}
			for _, org := range orgs {
				w.fetchGitHubOrg(org.Login, excludeList)
			}
		}
	}

	w.printFetchSummary()
	return nil
}

func (w *wizardState) fetchGitHubOrg(orgName string, excludeList []string) {
	orgRepos, err := gh.GetOrgRepos(w.token, orgName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not fetch repos for org %s: %v\n", orgName, err)
		return
	}

	var entries []repoEntry
	for _, r := range orgRepos {
		if w.shouldExcludeGH(r, excludeList, orgName) {
			continue
		}
		entries = append(entries, repoEntry{name: r.Name, cloneURL: r.CloneURL})
	}
	if len(entries) > 0 {
		w.orgGroups = append(w.orgGroups, orgWithRepos{name: orgName, repos: entries})
	}
}

func (w *wizardState) fetchGitLabRepos(excludeList []string, fetchPersonal, fetchOrgs bool) error {
	if fetchPersonal {
		projects, err := gl.GetUserProjects(w.token, w.gitlabURL)
		if err != nil {
			return err
		}
		for _, p := range projects {
			if p.Namespace.Kind != "user" {
				continue
			}
			if w.shouldExcludeGL(p, excludeList, "") {
				continue
			}
			w.personalRepos = append(w.personalRepos, repoEntry{name: p.Name, cloneURL: p.HTTPURLToRepo})
		}
	}

	if fetchOrgs {
		glGroups, err := gl.GetUserGroups(w.token, w.gitlabURL)
		if err != nil {
			return err
		}

		specificOrgs := flagOrg
		if len(specificOrgs) > 0 {
			orgSet := make(map[string]bool)
			for _, o := range specificOrgs {
				orgSet[strings.ToLower(o)] = true
			}
			for _, g := range glGroups {
				if !orgSet[strings.ToLower(g.Path)] && !orgSet[strings.ToLower(g.Name)] {
					continue
				}
				w.fetchGitLabGroup(g.ID, g.Path, excludeList)
			}
		} else {
			for _, g := range glGroups {
				w.fetchGitLabGroup(g.ID, g.Path, excludeList)
			}
		}
	}

	w.printFetchSummary()
	return nil
}

func (w *wizardState) fetchGitLabGroup(groupID int, groupPath string, excludeList []string) {
	projects, err := gl.GetGroupProjects(w.token, w.gitlabURL, groupID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not fetch projects for group %s: %v\n", groupPath, err)
		return
	}

	var entries []repoEntry
	for _, p := range projects {
		if w.shouldExcludeGL(p, excludeList, groupPath) {
			continue
		}
		entries = append(entries, repoEntry{name: p.Name, cloneURL: p.HTTPURLToRepo})
	}
	if len(entries) > 0 {
		w.orgGroups = append(w.orgGroups, orgWithRepos{name: groupPath, repos: entries})
	}
}

func (w *wizardState) printFetchSummary() {
	totalOrg := 0
	for _, og := range w.orgGroups {
		totalOrg += len(og.repos)
	}
	fmt.Printf("  Found %d personal repos", len(w.personalRepos))
	if len(w.orgGroups) > 0 {
		fmt.Printf(", %d repos across %d orgs", totalOrg, len(w.orgGroups))
	}
	fmt.Println("\n")
}

func (w *wizardState) shouldExcludeGH(r gh.Repo, excludeList []string, orgName string) bool {
	for _, e := range excludeList {
		if strings.EqualFold(r.Name, strings.TrimSpace(e)) {
			return true
		}
	}
	if w.cfg.IsExcluded(r.Name, orgName) {
		return true
	}
	if flagSkipArchived && r.Archived {
		return true
	}
	if flagSkipForks && r.Fork {
		return true
	}
	if w.visibility == "private" && !r.Private {
		return true
	}
	if w.visibility == "public" && r.Private {
		return true
	}
	return false
}

func (w *wizardState) shouldExcludeGL(p gl.Project, excludeList []string, groupPath string) bool {
	for _, e := range excludeList {
		if strings.EqualFold(p.Name, strings.TrimSpace(e)) {
			return true
		}
	}
	if w.cfg.IsExcluded(p.Name, groupPath) {
		return true
	}
	if flagSkipArchived && p.Archived {
		return true
	}
	if flagSkipForks && p.IsFork() {
		return true
	}
	if w.visibility == "private" && !p.IsPrivate() {
		return true
	}
	if w.visibility == "public" && !p.IsPublic() {
		return true
	}
	return false
}

// --- Step 7: Scope ---

func (w *wizardState) stepScope() error {
	// Check flags
	if w.cmd.Flags().Changed("only-orgs") && flagOnlyOrgs {
		w.scope = "orgs"
		return nil
	}
	if w.cmd.Flags().Changed("with-orgs") {
		if flagWithOrgs {
			w.scope = "everything"
		} else {
			w.scope = "personal"
		}
		return nil
	}
	if w.cmd.Flags().Changed("org") {
		w.scope = "everything"
		return nil
	}
	if w.cmd.Flags().Changed("pick") && flagPick {
		w.scope = "manual"
		return w.pickManually()
	}

	if !w.interactive {
		// Non-interactive default: personal only (old behavior)
		w.scope = "personal"
		return nil
	}

	hasPersonal := len(w.personalRepos) > 0
	hasOrgs := len(w.orgGroups) > 0

	if !hasPersonal && !hasOrgs {
		return fmt.Errorf("no repos found matching your criteria")
	}
	if !hasPersonal && hasOrgs {
		w.scope = "orgs"
		return nil
	}
	if hasPersonal && !hasOrgs {
		w.scope = "personal"
		return nil
	}

	// Both available
	totalOrgRepos := 0
	for _, og := range w.orgGroups {
		totalOrgRepos += len(og.repos)
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "What to clone?",
		Options: []string{
			fmt.Sprintf("Everything (%d personal + %d from %d orgs)", len(w.personalRepos), totalOrgRepos, len(w.orgGroups)),
			fmt.Sprintf("Personal repos only (%d)", len(w.personalRepos)),
			fmt.Sprintf("Organization repos only (%d)", totalOrgRepos),
			"Pick manually",
		},
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	switch {
	case strings.HasPrefix(choice, "Everything"):
		w.scope = "everything"
	case strings.HasPrefix(choice, "Personal"):
		w.scope = "personal"
	case strings.HasPrefix(choice, "Organization"):
		w.scope = "orgs"
	default:
		w.scope = "manual"
		return w.pickManually()
	}

	return nil
}

func (w *wizardState) pickManually() error {
	if len(w.personalRepos) > 0 {
		var names []string
		for _, r := range w.personalRepos {
			names = append(names, r.name)
		}
		var selected []string
		prompt := &survey.MultiSelect{
			Message: fmt.Sprintf("Personal repos (%d):", len(names)),
			Options: names,
			Default: names,
		}
		if err := survey.AskOne(prompt, &selected); err != nil {
			return err
		}
		set := make(map[string]bool)
		for _, s := range selected {
			set[s] = true
		}
		var filtered []repoEntry
		for _, r := range w.personalRepos {
			if set[r.name] {
				filtered = append(filtered, r)
			}
		}
		w.personalRepos = filtered
	}

	for i, og := range w.orgGroups {
		if len(og.repos) == 0 {
			continue
		}
		var names []string
		for _, r := range og.repos {
			names = append(names, r.name)
		}
		var selected []string
		prompt := &survey.MultiSelect{
			Message: fmt.Sprintf("Org: %s (%d repos):", og.name, len(names)),
			Options: names,
			Default: names,
		}
		if err := survey.AskOne(prompt, &selected); err != nil {
			return err
		}
		set := make(map[string]bool)
		for _, s := range selected {
			set[s] = true
		}
		var filtered []repoEntry
		for _, r := range og.repos {
			if set[r.name] {
				filtered = append(filtered, r)
			}
		}
		w.orgGroups[i].repos = filtered
	}

	return nil
}

// --- Step 8: Structure ---

func (w *wizardState) stepStructure() error {
	if w.legacyDest {
		return nil
	}

	// Check if we actually have orgs in the selection
	hasOrgs := false
	if w.scope == "everything" || w.scope == "orgs" || w.scope == "manual" {
		for _, og := range w.orgGroups {
			if len(og.repos) > 0 {
				hasOrgs = true
				break
			}
		}
	}

	if !hasOrgs {
		w.structure = "grouped"
		return nil
	}

	if w.cmd.Flags().Changed("flat") && flagFlat {
		w.structure = "flat"
		return nil
	}

	w.prompted = true
	var choice string
	prompt := &survey.Select{
		Message: "Folder structure?",
		Options: []string{
			fmt.Sprintf("Separated — orgs alongside, personal in \"%s/\"", w.folderName),
			fmt.Sprintf("Grouped — everything under \"%s/\"", w.folderName),
		},
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	if strings.HasPrefix(choice, "Separated") {
		w.structure = "separated"
	} else {
		w.structure = "grouped"
	}

	return nil
}

// --- Build Groups ---

func (w *wizardState) buildGroups() error {
	w.groups = nil

	if w.legacyDest {
		return w.buildLegacyGroups()
	}

	personalDest := filepath.Join(w.basePath, w.folderName)

	// Personal repos
	if w.scope != "orgs" && len(w.personalRepos) > 0 {
		w.groups = append(w.groups, repoGroup{
			label:   fmt.Sprintf("Personal Repos (%d repos)", len(w.personalRepos)),
			dest:    personalDest,
			entries: w.personalRepos,
		})
	}

	// Org repos
	if w.scope != "personal" {
		for _, og := range w.orgGroups {
			if len(og.repos) == 0 {
				continue
			}
			var orgDest string
			switch w.structure {
			case "separated":
				orgDest = filepath.Join(w.basePath, og.name)
			case "grouped":
				orgDest = filepath.Join(personalDest, og.name)
			case "flat":
				orgDest = personalDest
			default:
				orgDest = filepath.Join(w.basePath, og.name)
			}
			w.groups = append(w.groups, repoGroup{
				label:   fmt.Sprintf("Org: %s (%d repos)", og.name, len(og.repos)),
				dest:    orgDest,
				entries: og.repos,
			})
		}
	}

	return nil
}

func (w *wizardState) buildLegacyGroups() error {
	// Old behavior: personal -> basePath, orgs -> basePath/<org>
	if w.scope != "orgs" && len(w.personalRepos) > 0 {
		w.groups = append(w.groups, repoGroup{
			label:   fmt.Sprintf("Personal Repos (%d repos)", len(w.personalRepos)),
			dest:    w.basePath,
			entries: w.personalRepos,
		})
	}

	if w.scope != "personal" {
		for _, og := range w.orgGroups {
			if len(og.repos) == 0 {
				continue
			}
			orgDest := filepath.Join(w.basePath, og.name)
			if flagFlat {
				orgDest = w.basePath
			}
			w.groups = append(w.groups, repoGroup{
				label:   fmt.Sprintf("Org: %s (%d repos)", og.name, len(og.repos)),
				dest:    orgDest,
				entries: og.repos,
			})
		}
	}

	return nil
}

// --- Step 9: Summary ---

func (w *wizardState) stepSummary() error {
	if !w.prompted {
		return nil
	}

	totalRepos := 0
	for _, g := range w.groups {
		totalRepos += len(g.entries)
	}

	if totalRepos == 0 {
		return fmt.Errorf("no repos matched your criteria")
	}

	platformLabel := "GitHub"
	if w.platform == "gitlab" {
		platformLabel = "GitLab"
	}

	fmt.Println("━━ Summary ━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Platform:    %s\n", platformLabel)
	fmt.Printf("User:        %s (@%s)\n", w.displayName, w.login)
	if w.legacyDest {
		fmt.Printf("Destination: %s\n", w.basePath)
	} else {
		fmt.Printf("Path:        %s\n", w.basePath)
		fmt.Printf("Folder:      %s\n", w.folderName)
	}

	personalCount := 0
	if w.scope != "orgs" {
		personalCount = len(w.personalRepos)
	}
	orgRepoCount := totalRepos - personalCount

	repoDesc := fmt.Sprintf("%d", totalRepos)
	if personalCount > 0 && orgRepoCount > 0 {
		orgGroupCount := 0
		for _, og := range w.orgGroups {
			if len(og.repos) > 0 {
				orgGroupCount++
			}
		}
		repoDesc = fmt.Sprintf("%d (%d personal + %d from %d orgs)", totalRepos, personalCount, orgRepoCount, orgGroupCount)
	} else if personalCount > 0 {
		repoDesc = fmt.Sprintf("%d personal", personalCount)
	} else if orgRepoCount > 0 {
		orgGroupCount := 0
		for _, og := range w.orgGroups {
			if len(og.repos) > 0 {
				orgGroupCount++
			}
		}
		repoDesc = fmt.Sprintf("%d from %d orgs", orgRepoCount, orgGroupCount)
	}
	fmt.Printf("Repos:       %s\n", repoDesc)

	switch w.visibility {
	case "public":
		fmt.Println("Visibility:  Public only")
	case "private":
		fmt.Println("Visibility:  Private only")
	default:
		fmt.Println("Visibility:  All")
	}

	if orgRepoCount > 0 && !w.legacyDest {
		switch w.structure {
		case "separated":
			fmt.Println("Structure:   Separated")
		case "grouped":
			fmt.Println("Structure:   Grouped")
		}
	}

	if w.dryRun {
		fmt.Println("Mode:        Dry run")
	}

	fmt.Println()

	// Offer to expand
	var showList bool
	expandPrompt := &survey.Confirm{
		Message: "Show full repo list?",
		Default: false,
	}
	if err := survey.AskOne(expandPrompt, &showList); err != nil {
		return err
	}

	if showList {
		for _, g := range w.groups {
			fmt.Printf("\n  %s\n  -> %s\n", g.label, g.dest)
			for _, e := range g.entries {
				fmt.Printf("     %s\n", e.name)
			}
		}
		fmt.Println()
	}

	// Confirm
	var proceed bool
	confirmPrompt := &survey.Confirm{
		Message: "Proceed?",
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &proceed); err != nil {
		return err
	}

	if !proceed {
		fmt.Println("Cancelled.")
		os.Exit(0)
	}

	return nil
}
