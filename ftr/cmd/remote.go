package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote repositories",
	Long:  `Perform actions on remote repositories by interacting with the InkDrop server.`,
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "delete <user>/<repo>/<file>",
	Short: "Remove a file from a remote repository",
	Long:  `Permanently removes a file from a remote repository on the InkDrop server. This action requires you to be logged in and to be the owner of the repository.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]

		parts := strings.Split(repoPath, "/")
		if len(parts) != 3 {
			return fmt.Errorf("invalid path format. Must be <user>/<repo>/<file>")
		}
		user, repo, fileName := parts[0], parts[1], parts[2]

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		if !client.IsLoggedIn() {
			return fmt.Errorf("you must be logged in to remove files. Please run 'ftr login'")
		}

		fmt.Printf("Attempting to remove '%s' from %s...\n", fileName, repoPath)

		if err := client.DeleteRemoteFile(user, repo, fileName); err != nil {
			return fmt.Errorf("failed to remove file: %w", err)
		}

		fmt.Printf("Successfully removed '%s' from repository %s.\n", fileName, repoPath)
		return nil
	},
}

var remoteDownCmd = &cobra.Command{
	Use:   "down <user>/<repo>/<file-path>",
	Short: "Download a single file from a remote repository",
	Long:  `Downloads a specific file from a remote repository to the current directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fullPath := args[0]

		parts := strings.SplitN(fullPath, "/", 3)
		if len(parts) < 3 {
			return fmt.Errorf("invalid path format. Must be <user>/<repo>/<file-path>")
		}
		user, repo, filePath := parts[0], parts[1], parts[2]
		repoPath := fmt.Sprintf("%s/%s", user, repo)

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		destPath := filepath.Base(filePath)
		fmt.Printf("Downloading '%s' from %s to '%s'...\n", filePath, repoPath, destPath)

		if err := client.DownloadAndVerify(user, repo, filePath, destPath, nil); err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}

		fmt.Printf("\nSuccessfully downloaded %s.\n", destPath)
		return nil
	},
}

func init() {
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteDownCmd)
}
