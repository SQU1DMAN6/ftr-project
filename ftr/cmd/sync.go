package cmd

import (
	"bufio"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/screen"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync <user>/<repo> [flags]",
	Short: "Synchronise files with a repository",
	Long: `Snchronise files between your local sync directory and a remote repository.
Files are compared by name and timestamp to detect conflicts.

Examples:
  ftr sync qchef/media
  ftr sync qchef/media -E
  ftr sync qchef/media -w 8`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in format user/repo")
		}
		user, repo := parts[0], parts[1]

		encrypt, _ := cmd.Flags().GetBool("encrypt")
		workers, _ := cmd.Flags().GetInt("workers")
		autoChoice, _ := cmd.Flags().GetString("auto")

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		// Get sync directory
		syncDir := filepath.Join(os.Getenv("HOME"), "FtRSync", user, repo)
		if err := os.MkdirAll(syncDir, 0755); err != nil {
			return fmt.Errorf("failed to create sync directory: %w", err)
		}

		fmt.Printf("Syncing %s/%s with %s\n", user, repo, syncDir)

		// List remote files
		remoteFiles, err := client.ListRepoFiles(user, repo)
		if err != nil {
			return screen.SuggestLoginError(fmt.Errorf("failed to list remote files: %w", err))
		}

		// List local files
		localFiles := make(map[string]os.FileInfo)
		if err := filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(syncDir, path)
			localFiles[rel] = info
			return nil
		}); err != nil {
			return fmt.Errorf("failed to scan local files: %w", err)
		}

		// Build task lists: uploads, downloads, conflicts
		uploads := []string{}   // local only or newer
		downloads := []string{} // remote only or newer
		conflicts := []string{} // both exist with different times

		remoteMap := make(map[string]map[string]interface{})
		for _, rf := range remoteFiles {
			if path, ok := rf["path"].(string); ok {
				remoteMap[path] = rf
			}
		}

		// Find uploads and conflicts
		for localPath := range localFiles {
			if _, exists := remoteMap[localPath]; exists {
				// File exists both sides - check timestamps
				localTime := localFiles[localPath].ModTime().Unix()
				if modified, ok := remoteMap[localPath]["modified"].(float64); ok {
					// Compare Unix timestamps
					if int64(modified) != localTime {
						conflicts = append(conflicts, localPath)
					}
				}
			} else {
				// Local only - upload
				uploads = append(uploads, localPath)
			}
		}

		// Find downloads
		for remoteName := range remoteMap {
			if _, exists := localFiles[remoteName]; !exists {
				downloads = append(downloads, remoteName)
			}
		}

		fmt.Printf("Local only (will upload): %d\n", len(uploads))
		fmt.Printf("Remote only (will download): %d\n", len(downloads))
		fmt.Printf("Conflicts: %d\n", len(conflicts))

		// Handle conflicts: support automatic resolution via --auto or interactive prompts
		switch autoChoice {
		case "local":
			// prefer local: upload all conflicts
			for _, c := range conflicts {
				uploads = append(uploads, c)
			}
		case "remote":
			// prefer remote: download all conflicts
			for _, c := range conflicts {
				downloads = append(downloads, c)
			}
		case "both":
			// keep both: download remote and also upload (so caller can rename)
			for _, c := range conflicts {
				downloads = append(downloads, c)
				uploads = append(uploads, c)
			}
		case "skip":
			// do nothing
		default:
			// Interactive prompts with optional 'apply to all'
			applyAll := false
			var applyAction string
			for _, conflict := range conflicts {
				if applyAll {
					switch applyAction {
					case "u":
						uploads = append(uploads, conflict)
					case "d":
						downloads = append(downloads, conflict)
					case "b":
						downloads = append(downloads, conflict)
						uploads = append(uploads, conflict)
					case "s":
						// skip
					}
					continue
				}

				// Show helpful info about the conflicting files
				fmt.Printf("Conflict: %s\n", conflict)
				// local info
				if lf, ok := localFiles[conflict]; ok {
					fmt.Printf("  Local: %s (modified: %s, size: %d)\n", conflict, lf.ModTime().Format("2006-01-02 15:04:05"), lf.Size())
				}
				// remote info
				if rf, ok := remoteMap[conflict]; ok {
					if m, ok := rf["modified"].(float64); ok {
						t := time.Unix(int64(m), 0).Format("2006-01-02 15:04:05")
						sz := int64(0)
						if s, ok := rf["size"].(float64); ok {
							sz = int64(s)
						}
						fmt.Printf("  Remote: %s (modified: %s, size: %d)\n", conflict, t, sz)
					}
				}

				// Suggest a default based on which side is newer
				defaultChoice := "u"
				if lf, ok := localFiles[conflict]; ok {
					if rf, ok := remoteMap[conflict]; ok {
						if m, ok := rf["modified"].(float64); ok {
							localTime := lf.ModTime().Unix()
							remoteTime := int64(m)
							if remoteTime > localTime {
								defaultChoice = "d"
							} else {
								defaultChoice = "u"
							}
						}
					}
				}

				fmt.Printf("  Choose: [u]pload local, [d]ownload remote, [s]kip, [b]oth (keep both) (default: %s): ", defaultChoice)
				reader := bufio.NewReader(os.Stdin)
				choice, _ := reader.ReadString('\n')
				choice = strings.TrimSpace(choice)
				if choice == "" {
					choice = defaultChoice
				}

				switch choice {
				case "u":
					uploads = append(uploads, conflict)
				case "d":
					downloads = append(downloads, conflict)
				case "b":
					downloads = append(downloads, conflict)
					uploads = append(uploads, conflict)
				case "s":
					// skip
				default:
					fmt.Println("Unrecognized choice; skipping")
				}

				// Ask whether to apply to all remaining
				fmt.Printf("  Apply this action to all remaining conflicts? [y/N]: ")
				resp, _ := reader.ReadString('\n')
				resp = strings.TrimSpace(resp)
				if strings.EqualFold(resp, "y") {
					applyAll = true
					// determine last action
					if choice == "u" || choice == "d" || choice == "b" || choice == "s" {
						applyAction = choice
					}
				}
			}
		}

		// Execute parallel transfers with worker pool
		uploadChan := make(chan string, len(uploads))
		downloadChan := make(chan string, len(downloads))
		var wg sync.WaitGroup

		// Spawn workers
		for i := 0; i < workers; i++ {
			wg.Add(2)

			// Upload worker
			go func() {
				defer wg.Done()
				for localPath := range uploadChan {
					fullPath := filepath.Join(syncDir, localPath)
					if file, err := os.Open(fullPath); err == nil {
						err = client.UploadFile(repoPath, localPath, file, encrypt)
						file.Close()
						if err != nil {
							fmt.Printf("\n✗ Upload failed: %s: %v\n", localPath, err)
						} else {
							fmt.Printf("\n✓ Uploaded: %s\n", localPath)
						}
					} else {
						fmt.Printf("\n✗ Cannot open: %s: %v\n", localPath, err)
					}
				}
			}()

			// Download worker
			go func() {
				defer wg.Done()
				for remoteName := range downloadChan {
					destPath := filepath.Join(syncDir, remoteName)
					os.MkdirAll(filepath.Dir(destPath), 0755)
					err := client.DownloadAndVerify(repoPath, remoteName, destPath)
					if err != nil {
						fmt.Printf("\n✗ Download failed: %s: %v\n", remoteName, err)
					} else {
						fmt.Printf("\n✓ Downloaded: %s\n", remoteName)
					}
				}
			}()
		}

		// Queue tasks
		for _, path := range uploads {
			uploadChan <- path
		}
		close(uploadChan)

		for _, path := range downloads {
			downloadChan <- path
		}
		close(downloadChan)

		// Wait for completion
		wg.Wait()

		fmt.Printf("\nSync complete!\n")
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolP("encrypt", "E", false, "Encrypt uploaded files")
	syncCmd.Flags().IntP("workers", "w", 4, "Number of parallel workers")
	syncCmd.Flags().String("auto", "ask", "Auto-resolve conflicts: ask|local|remote|skip|both")
}
