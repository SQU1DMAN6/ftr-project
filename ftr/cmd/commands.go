package cmd

// This file ensures all commands are properly imported and registered

import (
	_ "fmt"           // Used by command implementations
	_ "io"            // Used by command implementations
	_ "os"            // Used by command implementations
	_ "path/filepath" // Used by command implementations
	_ "strings"       // Used by command implementations
)

// Commands registers all available commands
func init() {
	rootCmd.AddCommand(
		getCmd,
		upCmd,
		clearCmd,
		removeCmd,
		loginCmd,
		logoutCmd,
		sessionCmd,
		packCmd,
		buildCmd,
	)
}
