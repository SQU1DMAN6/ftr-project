package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var upEncrypt bool

func init() {
	// Register the -E flag for encrypted upload
	upCmd.Flags().BoolVarP(&upEncrypt, "encrypt", "E", false, "Encrypt file on upload")
}

var upCmd = &cobra.Command{
	Use:   "up [file] [user/repo]",
	Short: "Upload a file to a repository",
	Long: `Upload a file to a repository on the InkDrop server.

Example: ftr up myfile.txt user/repo`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// last arg is target repo, preceding args are source files
		repoPath := args[len(args)-1]
		rawSources := args[:len(args)-1]

		// Deduplicate sources while preserving order
		seen := make(map[string]struct{})
		sources := []string{}
		for _, s := range rawSources {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			sources = append(sources, s)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Parallel upload with a small concurrency limit
		sem := make(chan struct{}, 6)
		errCh := make(chan error, len(sources))

		for _, sourcePath := range sources {
			sourcePath := sourcePath
			info, err := os.Stat(sourcePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to access source path %s: %v\n", sourcePath, err)
				errCh <- err
				continue
			}
			if info.IsDir() {
				fmt.Fprintf(os.Stderr, "source must be a file, not a directory: %s\n", sourcePath)
				errCh <- fmt.Errorf("source is directory")
				continue
			}

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()

				fmt.Printf("Uploading %s to %s...\n", sourcePath, repoPath)
				f, err := os.Open(sourcePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open file %s: %v\n", sourcePath, err)
					errCh <- err
					return
				}
				defer f.Close()

				if err := client.UploadFile(repoPath, filepath.Base(sourcePath), f, info.Size(), upEncrypt, nil); err != nil {
					fmt.Fprintf(os.Stderr, "upload failed for %s: %v\n", sourcePath, err)
					errCh <- err
					return
				}

				fmt.Printf("File %s uploaded successfully\n", filepath.Base(sourcePath))
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
		return lastErr
	},
}
