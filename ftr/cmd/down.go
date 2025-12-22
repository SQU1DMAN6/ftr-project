package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var downDest string
var downWorkers int

func init() {
	downCmd.Flags().StringVarP(&downDest, "dest", "D", "", "Destination directory (defaults to ~/FtRSync)")
	downCmd.Flags().IntVarP(&downWorkers, "workers", "w", 10, "Number of parallel workers")
}

var downCmd = &cobra.Command{
	Use:   "down [user/repo]",
	Short: "Download all files from a repository into the FtR home dir",
	Long:  "Recursively download every file in the repo into ~/FtRSync or the directory provided with -D",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository path. Must be in format user/repo")
		}
		user := parts[0]
		repo := parts[1]

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// destination directory
		var dest string
		if downDest != "" {
			dest = downDest
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}
			dest = filepath.Join(home, "FtRSync", user, repo)
		}

		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		fmt.Printf("Listing files in %s/%s...\n", user, repo)
		files, err := client.ListRepoFiles(user, repo)
		if err != nil {
			return fmt.Errorf("failed to list repo files: %w", err)
		}

		if len(files) == 0 {
			fmt.Println("No files found.")
			return nil
		}

		errorsList := []string{}
		var errMu sync.Mutex

		type downloadTask struct {
			pathRel  string
			fullPath string
		}
		tasks := make(chan downloadTask, len(files))
		var wg sync.WaitGroup

		// Start workers
		for i := 0; i < downWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					if err := client.DownloadAndVerify(user, repo, task.pathRel, task.fullPath, nil); err != nil {
						errMu.Lock()
						errorsList = append(errorsList, fmt.Sprintf("failed to download %s: %v", task.pathRel, err))
						errMu.Unlock()
					}
				}
			}()
		}

		// Queue tasks
		for _, f := range files {
			pathRel, _ := f["path"].(string)
			if pathRel == "" {
				continue
			}
			// ensure subdirs
			fullPath := filepath.Join(dest, filepath.FromSlash(pathRel))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("failed to create dir for %s: %w", fullPath, err)
			}

			tasks <- downloadTask{pathRel: pathRel, fullPath: fullPath}
		}
		close(tasks)

		wg.Wait()

		if len(errorsList) > 0 {
			fmt.Printf("\nErrors encountered during download:\n")
			for _, e := range errorsList {
				fmt.Printf("- %s\n", e)
			}
		}

		fmt.Println("All files processed.")
		return nil
	},
}
