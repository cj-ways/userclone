package cmd

import (
	"fmt"
	"strings"

	"github.com/cj-ways/userclone/internal/config"
	"github.com/spf13/cobra"
)

var defaultCmd = &cobra.Command{
	Use:   "default <setting> <value>",
	Short: "Set a persistent default setting",
	Long: `Set persistent defaults in ~/.userclone.yml.

Available settings:
  platform    github or gitlab
  dest        default destination directory
  with-orgs   true or false`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		setting := strings.ToLower(args[0])
		value := args[1]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		switch setting {
		case "platform":
			if value != "github" && value != "gitlab" {
				return fmt.Errorf("platform must be 'github' or 'gitlab'")
			}
			cfg.DefaultPlatform = value
		case "dest":
			cfg.DefaultDest = value
		case "with-orgs":
			switch strings.ToLower(value) {
			case "true", "yes", "1":
				cfg.WithOrgs = true
			case "false", "no", "0":
				cfg.WithOrgs = false
			default:
				return fmt.Errorf("with-orgs must be true or false")
			}
		default:
			return fmt.Errorf("unknown setting: %s\nAvailable: platform, dest, with-orgs", setting)
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Default %s set to: %s\n", setting, value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(defaultCmd)
}
