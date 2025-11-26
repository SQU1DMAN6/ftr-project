package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo]",
	Short: "Remove an installed package",
	Long: `Remove an installed package from the system.
This will remove the binary from /usr/local/bin and its directory from /usr/share.

Example: ftr remove myapp
         ftr remove user/myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]

		// Extract repo name if full path is given
		repoName := repoPath
		if strings.Contains(repoPath, "/") {
			parts := strings.Split(repoPath, "/")
			repoName = parts[len(parts)-1]
		}

		binPath := filepath.Join("/usr/local/bin", repoName)
		sharePath := filepath.Join("/usr/share", repoName)
		desktopEntry := filepath.Join("/usr/share/applications", repoName+".desktop")

		// Remove binary
		if _, err := os.Stat(binPath); err == nil {
			if err := exec.Command("sudo", "rm", "-rf", binPath).Run(); err != nil {
				return fmt.Errorf("failed to remove binary: %w", err)
			}
			fmt.Printf("Removed binary from %s\n", binPath)
		} else {
			fmt.Println("Binary not found in /usr/local/bin")
		}

		// Remove share directory
		if _, err := os.Stat(sharePath); err == nil {
			if err := exec.Command("sudo", "rm", "-rf", sharePath).Run(); err != nil {
				return fmt.Errorf("failed to remove share directory: %w", err)
			}
			fmt.Printf("Removed directory %s\n", sharePath)
		} else {
			fmt.Println("Share directory not found")
		}

		// Remove desktop entry
		if _, err := os.Stat(desktopEntry); err == nil {
			if err := exec.Command("sudo", "rm", "-f", desktopEntry).Run(); err != nil {
				return fmt.Errorf("failed to remove desktop entry: %w", err)
			}
			fmt.Printf("Removed desktop entry at %s\n", desktopEntry)
		} else {
			fmt.Println("Desktop entry was not found.")
		}

		return nil
	},
}
