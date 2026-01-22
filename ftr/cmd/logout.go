package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	// No need to register
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your FtR session",
	Long:  "Removes all saved session data, effectively logging you out of FtR.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		configDir := filepath.Join(home, ".config", "ftr")
		sessionFile := filepath.Join(configDir, "session")
		emailFile := filepath.Join(configDir, "email")
		usernameFile := filepath.Join(configDir, "username")

		// Delete the session file if it exists
		if _, err := os.Stat(sessionFile); err == nil {
			if err := os.Remove(sessionFile); err != nil {
				return fmt.Errorf("failed to remove session file: %w", err)
			}
		}

		// Delete the email file if it exists
		if _, err := os.Stat(emailFile); err == nil {
			if err := os.Remove(emailFile); err != nil {
				return fmt.Errorf("failed to remove email file: %w", err)
			}
		}

		// Delete the username file if it exists
		if _, err := os.Stat(usernameFile); err == nil {
			if err := os.Remove(usernameFile); err != nil {
				return fmt.Errorf("failed to remove username file: %w", err)
			}
		}

		if _, err := os.Stat(sessionFile); err == nil {
			fmt.Println("Logged out successfully.")
		}

		// Optionally, remove the config directory if empty
		_ = os.Remove(configDir)

		return nil
	},
}
