package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ftr/pkg/registry"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo]...",
	Short: "Remove an installed package",
	Long: `Remove an installed package from the system.
This will remove the binary from /usr/local/bin and its directory from /usr/local/share.

Examples:
  ftr remove myapp
  ftr remove user/myapp otherapp`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var lastErr error
		for _, repoPath := range args {
			// Extract repo name if full path is given
			repoName := repoPath
			if strings.Contains(repoPath, "/") {
				parts := strings.Split(repoPath, "/")
				repoName = parts[len(parts)-1]
			}

			binPath := filepath.Join("/usr/local/bin", repoName)
			sharePath := filepath.Join("/usr/local/share", repoName)
			desktopEntry := filepath.Join("/usr/local/share/applications", repoName+".desktop")

			// Remove binary (silent if not found)
			if _, err := os.Stat(binPath); err == nil {
				if err := exec.Command("sudo", "rm", "-rf", binPath).Run(); err != nil {
					lastErr = fmt.Errorf("failed to remove binary %s: %w", binPath, err)
					fmt.Fprintln(os.Stderr, lastErr)
				} else {
					fmt.Printf("Removed binary from %s\n", binPath)
				}
			}

			// Remove share directory (silent if not found)
			if _, err := os.Stat(sharePath); err == nil {
				if err := exec.Command("sudo", "rm", "-rf", sharePath).Run(); err != nil {
					lastErr = fmt.Errorf("failed to remove share directory %s: %w", sharePath, err)
					fmt.Fprintln(os.Stderr, lastErr)
				} else {
					fmt.Printf("Removed directory %s\n", sharePath)
				}
			}

			// Remove desktop entry (silent if not found)
			if _, err := os.Stat(desktopEntry); err == nil {
				if err := exec.Command("sudo", "rm", "-f", desktopEntry).Run(); err != nil {
					lastErr = fmt.Errorf("failed to remove desktop entry %s: %w", desktopEntry, err)
					fmt.Fprintln(os.Stderr, lastErr)
				} else {
					fmt.Printf("Removed desktop entry at %s\n", desktopEntry)
				}
			}

			// Unregister from registry if present (silent if not found)
			if err := registry.Unregister(repoName); err != nil {
				// Silently ignore "package not found" errors, but report other errors
				if err.Error() != "package not found" {
					lastErr = err
					fmt.Fprintln(os.Stderr, err)
				}
			}
		}
		return lastErr
	},
}
