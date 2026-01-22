package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "View your current session information",
	Long:  `Display your current FtR session information including email and username.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		ok, err := client.SessionConfirmed()
		if err != nil {
			return fmt.Errorf("failed to verify session: %w", err)
		}

		if !ok {
			home, err := os.UserHomeDir()
			if err == nil {
				configDir := filepath.Join(home, ".config", "ftr")
				sessionFile := filepath.Join(configDir, "session")
				emailFile := filepath.Join(configDir, "email")
				usernameFile := filepath.Join(configDir, "username")
				_ = os.Remove(sessionFile)
				_ = os.Remove(emailFile)
				_ = os.Remove(usernameFile)
				_ = os.Remove(configDir)
			}
			fmt.Println("No active session on server. Please log in again.")
			return nil
		}

		email, username := client.GetSessionInfo()

		fmt.Println("Current Session Information:")
		if email != "" {
			fmt.Printf("    Email       %s\n", email)
		} else {
			fmt.Printf("    Email       %s\n", "(unknown)")
		}
		if username != "" {
			fmt.Printf("    Username    %s\n", username)
		} else {
			fmt.Printf("    Username    %s\n", "(unknown)")
		}

		return nil
	},
}
