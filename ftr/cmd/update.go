package cmd

import (
	"fmt"
	"ftr/pkg/registry"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all installed packages",
	Long:  "Checks installed packages and attempts to fetch and install their latest versions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgs, err := registry.List()
		if err != nil {
			return fmt.Errorf("failed to list installed packages: %w", err)
		}
		var lastErr error
		for _, p := range pkgs {
			if p.Source == "" {
				fmt.Printf("Skipping %s (no source metadata)\n", p.Name)
				continue
			}
			fmt.Printf("Updating %s...\n", p.Name)
			if err := getCmd.RunE(getCmd, []string{p.Source}); err != nil {
				lastErr = err
				fmt.Printf("Failed to update %s: %v\n", p.Name, err)
			}
		}
		return lastErr
	},
}
