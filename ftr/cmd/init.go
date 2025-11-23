package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize local FtR sync directory",
	Long:  "Creates a local directory `~/FtRSync` for syncing packages (like Dropbox).",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}
		dir := filepath.Join(home, "FtRSync")
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dir, err)
			}
			fmt.Printf("Created directory: %s\n", dir)
		} else {
			fmt.Printf("Directory already exists: %s\n", dir)
		}

		// Ensure config dir exists as well
		cfg := filepath.Join(home, ".config", "ftr")
		_ = os.MkdirAll(cfg, 0755)
		fmt.Printf("Config directory ensured: %s\n", cfg)
		return nil
	},
}
