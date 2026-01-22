package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/screen"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <user>/<repo>",
	Short: "List files in a remote repository",
	Long:  `Lists all files in a remote repository, showing their path, size, and modification time.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in format user/repo")
		}
		user, repo := parts[0], parts[1]

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		fmt.Printf("Querying repository: %s\n\n", repoPath)

		files, err := client.ListRepoFiles(user, repo)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "invalid character '<'") {
				// This happens when the server returns HTML instead of JSON
				if strings.Contains(errMsg, "response started with:") {
					// Extract what the response started with for more helpful error message
					return fmt.Errorf("repository %s not found or access denied - the server returned an error page\nTry checking the repository name or run 'ftr login' to authenticate", repoPath)
				}
				return screen.SuggestLoginError(fmt.Errorf("failed to list repo files: %w", err))
			}
			if strings.Contains(errMsg, "404") {
				return fmt.Errorf("repository '%s' not found", repoPath)
			}
			if strings.Contains(errMsg, "403") {
				return fmt.Errorf("access denied to repository '%s' - you don't have permission to access it", repoPath)
			}
			return fmt.Errorf("failed to list remote files: %w", err)
		}

		if len(files) == 0 {
			fmt.Println("Repository is empty or does not exist.")
			return nil
		}

		// Header
		termWidth := screen.TermWidth()
		pathWidth := int(float64(termWidth) * 0.5) // 50% for path
		if pathWidth < 20 {
			pathWidth = 20
		}
		headerFmt := fmt.Sprintf("%%-%ds %%12s   %%s\n", pathWidth)
		fmt.Printf(headerFmt, "NAME", "SIZE", "MODIFIED")
		fmt.Println(strings.Repeat("=", termWidth))

		for _, file := range files {
			path := file["path"].(string)
			size := int64(file["size"].(float64))
			modified := int64(file["modified"].(float64))
			modTime := time.Unix(modified, 0).Format("2006-01-02 15:04:05")

			// Truncate path if it's too long
			if len(path) > pathWidth {
				path = "..." + path[len(path)-pathWidth+3:]
			}

			fmt.Printf(headerFmt, path, formatSize(size), modTime)
		}

		return nil
	},
}

// formatSize converts bytes into a human-readable string (KB, MB, GB).
func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	const unit = 1024
	if size < unit*unit {
		return fmt.Sprintf("%.1f KB", float64(size)/unit)
	}
	if size < unit*unit*unit {
		return fmt.Sprintf("%.1f MB", float64(size)/float64(unit*unit))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/float64(unit*unit*unit))
}

func init() {
	// No flags for this command yet
}
