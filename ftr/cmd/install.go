package cmd

import (
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [user/repo]",
	Short: "Install a package (alias to get)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reuse get command implementation
		return getCmd.RunE(getCmd, args)
	},
}
