package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"ftr/pkg/boxlet"
)

var boxletCmd = &cobra.Command{
	Use:   "boxlet",
	Short: "Boxlet helper commands (create metadata etc.)",
}

var boxletInitCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize an FtR project and create BUILD/Meta.config",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}

		// Resolve path
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}

		// Collect flags
		name, _ := cmd.Flags().GetString("name")
		version, _ := cmd.Flags().GetString("version")
		arch, _ := cmd.Flags().GetString("arch")
		osName, _ := cmd.Flags().GetString("os")
		desc, _ := cmd.Flags().GetString("description")

		if name == "" {
			// fallback to directory name
			name = filepath.Base(absDir)
		}

		meta := boxlet.MetaKeyValue{
			"PACKAGE_NAME":        name,
			"VERSION":             version,
			"TARGET_ARCHITECTURE": arch,
			"TARGET_OS":           osName,
			"DESCRIPTION":         desc,
		}

		if err := boxlet.WriteMeta(absDir, meta); err != nil {
			return fmt.Errorf("failed to write metadata: %w", err)
		}

		fmt.Printf("Created BUILD/Meta.config for '%s'\n", absDir)
		return nil
	},
}

func init() {
	boxletInitCmd.Flags().StringP("name", "n", "", "Package name (defaults to directory name)")
	boxletInitCmd.Flags().StringP("version", "v", "0.0.1", "Package version")
	boxletInitCmd.Flags().StringP("arch", "a", "", "Target architecture (e.g., amd64, arm64)")
	boxletInitCmd.Flags().StringP("os", "s", "", "Target OS (e.g., linux, darwin)")
	boxletInitCmd.Flags().StringP("description", "d", "", "Short package description")

	boxletCmd.AddCommand(boxletInitCmd)
}
