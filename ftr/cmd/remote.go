package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"os"
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
	Use:   "delete <user>/<repo>/<file> [<user>/<repo>/<file>...]",
	Short: "Remove one or more files from a remote repository",
	Long: `Permanently removes one or more files from a remote repository on the InkDrop server. This action requires you to be logged in and to be the owner of the repository.

Examples:
  ftr remote delete user/repo/file.txt
  ftr remote delete user/repo/file1.txt user/repo/file2.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deduplicate file paths while preserving order
		seen := make(map[string]struct{})
		filePaths := []string{}
		for _, path := range args {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			filePaths = append(filePaths, path)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		if !client.IsLoggedIn() {
			return fmt.Errorf("you must be logged in to remove files. Please run 'ftr login'")
		}

		// Parallel deletion with a small concurrency limit
		sem := make(chan struct{}, 6)
		errCh := make(chan error, len(filePaths))

		for _, filePath := range filePaths {
			filePath := filePath
			parts := strings.Split(filePath, "/")
			if len(parts) != 3 {
				fmt.Fprintf(os.Stderr, "invalid path format: %s (must be <user>/<repo>/<file>)\n", filePath)
				errCh <- fmt.Errorf("invalid path format")
				continue
			}
			user, repo, fileName := parts[0], parts[1], parts[2]

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()

				fmt.Printf("Removing '%s' from %s/%s...\n", fileName, user, repo)

				if err := client.DeleteRemoteFile(user, repo, fileName); err != nil {
					fmt.Fprintf(os.Stderr, "failed to remove %s: %v\n", filePath, err)
					errCh <- err
					return
				}

				fmt.Printf("Successfully removed '%s' from %s/%s.\n", fileName, user, repo)
				errCh <- nil
			}()
		}

		// wait for all goroutines to finish
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}
		close(errCh)

		var lastErr error
		for e := range errCh {
			if e != nil {
				lastErr = e
			}
		}

		if lastErr != nil {
			return fmt.Errorf("one or more deletions failed")
		}
		return nil
	},
}

var remoteDownCmd = &cobra.Command{
	Use:   "down <user>/<repo>/<file-path> [<user>/<repo>/<file-path>...]",
	Short: "Download one or more files from a remote repository",
	Long: `Downloads specific files from a remote repository to the current directory.

Examples:
  ftr remote down user/repo/file.txt
  ftr remote down user/repo/file1.txt user/repo/file2.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deduplicate file paths while preserving order
		seen := make(map[string]struct{})
		filePaths := []string{}
		for _, path := range args {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			filePaths = append(filePaths, path)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Parallel download with a small concurrency limit
		sem := make(chan struct{}, 6)
		errCh := make(chan error, len(filePaths))

		for _, fullPath := range filePaths {
			fullPath := fullPath
			parts := strings.SplitN(fullPath, "/", 3)
			if len(parts) < 3 {
				fmt.Fprintf(os.Stderr, "invalid path format: %s (must be <user>/<repo>/<file-path>)\n", fullPath)
				errCh <- fmt.Errorf("invalid path format")
				continue
			}
			user, repo, filePath := parts[0], parts[1], parts[2]
			repoPath := fmt.Sprintf("%s/%s", user, repo)

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()

				destPath := filepath.Base(filePath)
				fmt.Printf("Downloading '%s' from %s to '%s'...\n", filePath, repoPath, destPath)

				if err := client.DownloadAndVerify(user, repo, filePath, destPath, nil); err != nil {
					fmt.Fprintf(os.Stderr, "failed to download %s: %v\n", fullPath, err)
					errCh <- err
					return
				}

				fmt.Printf("Successfully downloaded %s.\n", destPath)
				errCh <- nil
			}()
		}

		// wait for all goroutines to finish
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}
		close(errCh)

		var lastErr error
		for e := range errCh {
			if e != nil {
				lastErr = e
			}
		}

		if lastErr != nil {
			return fmt.Errorf("one or more downloads failed")
		}
		return nil
	},
}

func init() {
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteDownCmd)
}
