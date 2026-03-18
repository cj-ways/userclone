package cmd

import (
	"fmt"
	"os"

	"github.com/cj-ways/userclone/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a ~/.userclone.yml config template",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ConfigPath()
		if path == "" {
			return fmt.Errorf("could not determine home directory")
		}

		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Config file already exists at %s\n", path)
			fmt.Println("Delete it first if you want to regenerate.")
			return nil
		}

		template := config.GenerateTemplate()
		if err := os.WriteFile(path, []byte(template), 0600); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Printf("Config file created at %s\n", path)
		fmt.Println("Edit it to set your token and preferences.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
