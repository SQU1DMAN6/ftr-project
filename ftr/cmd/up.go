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
		sources := args[:len(args)-1]

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		var lastErr error
		for _, sourcePath := range sources {
			info, err := os.Stat(sourcePath)
			if err != nil {
				lastErr = fmt.Errorf("failed to access source path %s: %w", sourcePath, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}
			if info.IsDir() {
				lastErr = fmt.Errorf("source must be a file, not a directory: %s", sourcePath)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			fmt.Printf("Uploading %s to %s...\n", sourcePath, repoPath)
			f, err := os.Open(sourcePath)
			if err != nil {
				lastErr = fmt.Errorf("failed to open file %s: %w", sourcePath, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			if err := client.UploadFile(repoPath, filepath.Base(sourcePath), f, info.Size(), upEncrypt, nil); err != nil {
				lastErr = fmt.Errorf("upload failed for %s: %w", sourcePath, err)
				fmt.Fprintln(os.Stderr, lastErr)
				f.Close()
				continue
			}
			f.Close()

			fmt.Printf("File %s uploaded successfully\n", filepath.Base(sourcePath))
		}

		return lastErr
	},
}
