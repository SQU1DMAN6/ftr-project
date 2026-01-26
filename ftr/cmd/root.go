package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ftr",
	Short: "FtR - Package Manager, written by Quan Thai",
	Long: `FtR is a command-line tool for managing file repositories
and packages using the FSDL and SQAR format. It integrates with InkDrop for file sharing.`,
}

func Execute() error {
	return rootCmd.Execute()
}
