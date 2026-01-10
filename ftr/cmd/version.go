package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "View FtR's version on your system",
	Long:  `Display the version and release of FtR installed on your system, including package name, release name, and version number.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var version string = "FtR version 2.6"
		var release string = "Written by Quan Thai, November 22 2025"
		fmt.Println(version)
		fmt.Println(release)

		return nil
	},
}
