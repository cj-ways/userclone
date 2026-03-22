package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cj-ways/userclone/internal/config"
	"github.com/cj-ways/userclone/internal/git"
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
	Long: `Interactive wizard to clone your repos.

Walks you through: platform, authentication, destination,
visibility, scope, and folder structure.

Flags can pre-fill answers and skip wizard steps.
Provide all flags for fully non-interactive usage.`,
	RunE: runClone,
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
	cloneCmd.Flags().StringVarP(&flagDest, "dest", "d", "", "Base destination directory")
	cloneCmd.Flags().BoolVar(&flagWithOrgs, "with-orgs", false, "Clone repos from all orgs")
	cloneCmd.Flags().StringArrayVarP(&flagOrg, "org", "o", nil, "Clone repos from specific org(s)")
	cloneCmd.Flags().BoolVar(&flagOnlyOrgs, "only-orgs", false, "Skip personal repos")
	cloneCmd.Flags().StringVarP(&flagExclude, "exclude", "e", "", "Comma-separated repo names to skip")
	cloneCmd.Flags().BoolVar(&flagSkipArchived, "skip-archived", false, "Skip archived repos")
	cloneCmd.Flags().BoolVar(&flagSkipForks, "skip-forks", false, "Skip forked repos")
	cloneCmd.Flags().BoolVar(&flagPrivateOnly, "private-only", false, "Only private repos")
	cloneCmd.Flags().BoolVar(&flagPublicOnly, "public-only", false, "Only public repos")
	cloneCmd.Flags().BoolVar(&flagPick, "pick", false, "Interactive checkbox selection")
	cloneCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview without cloning")
	cloneCmd.Flags().BoolVar(&flagFlat, "flat", false, "No org grouping")
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

	groups, token, dryRun, err := runWizard(cmd, cfg)
	if err != nil {
		return err
	}

	if flagFlat {
		checkFlatCollisions(groups)
	}

	return executeClone(groups, token, dryRun)
}

func authenticatedCloneURL(cloneURL, token string) string {
	if token != "" && strings.HasPrefix(cloneURL, "https://") {
		return strings.Replace(cloneURL, "https://", "https://oauth2:"+token+"@", 1)
	}
	return cloneURL
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

func checkFlatCollisions(groups []repoGroup) {
	seen := make(map[string]string)
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
