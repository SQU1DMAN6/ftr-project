package cmd

import (
	"fmt"
	"ftr/pkg/api"

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

		email, username := client.GetSessionInfo()

		if email == "" || username == "" {
			return fmt.Errorf("no active session found. Please log in first with 'ftr login'")
		}

		fmt.Println("Current Session Information:")
		fmt.Printf("    Email       %s\r\n", email)
		fmt.Printf("    Username    %s\r\n", username)

		return nil
	},
}
