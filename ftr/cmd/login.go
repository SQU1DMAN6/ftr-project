package cmd

import (
	"bufio"
	"fmt"
	"ftr/pkg/api"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your account",
	Long: `Log in to your account to access remote repositories.
Required for uploading packages with 'up' command.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Support non-interactive login via flags
		emailFlag, _ := cmd.Flags().GetString("email")
		passwordFlag, _ := cmd.Flags().GetString("password")

		var email string
		var password string

		if emailFlag != "" && passwordFlag != "" {
			email = strings.TrimSpace(emailFlag)
			password = passwordFlag
		} else {
			reader := bufio.NewReader(os.Stdin)

			// Get email
			fmt.Print("Email: ")
			e, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read email: %w", err)
			}
			email = strings.TrimSpace(e)

			// Get password securely
			fmt.Print("Password: ")
			pwd, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			fmt.Println()
			password = string(pwd)
		}

		// Create API client and login
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		if err := client.Login(email, strings.TrimSpace(password)); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		fmt.Println("Successfully logged in")
		return nil
	},
}

func init() {
	loginCmd.Flags().StringP("email", "e", "", "Email address for non-interactive login")
	loginCmd.Flags().StringP("password", "p", "", "Password for non-interactive login (use with care)")
}
