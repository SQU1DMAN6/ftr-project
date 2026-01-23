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
This will remove the binary and share directory that were created during installation,
as recorded in the package registry. This ensures all installed files are removed.

Examples:
  ftr remove myapp
  ftr remove user/myapp otherapp`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var lastErr error
		for _, packageRef := range args {
			// Extract package name (handle both "repo" and "user/repo" formats)
			packageName := packageRef
			if strings.Contains(packageRef, "/") {
				parts := strings.Split(packageRef, "/")
				packageName = parts[len(parts)-1]
			}

			// Look up the package in the registry
			pkgInfo, err := registry.Find(packageName)
			if err != nil {
				// "not found" errors are just warnings, don't set lastErr
				fmt.Fprintf(os.Stderr, "Warning: package '%s' not found in registry\n", packageName)
				continue
			}

			// Remove the binary if it exists
			if pkgInfo.BinaryPath != "" {
				if _, err := os.Stat(pkgInfo.BinaryPath); err == nil {
					if err := exec.Command("sudo", "rm", "-f", pkgInfo.BinaryPath).Run(); err != nil {
						lastErr = fmt.Errorf("failed to remove binary %s: %w", pkgInfo.BinaryPath, err)
						fmt.Fprintln(os.Stderr, lastErr)
					} else {
						fmt.Printf("Removed binary at %s\n", pkgInfo.BinaryPath)
					}
				}
			}

			// Remove the share directory if it exists
			// InstallPath may contain multiple paths separated by colons (for install.sh packages)
			if pkgInfo.InstallPath != "" {
				paths := strings.Split(pkgInfo.InstallPath, ":")
				for _, path := range paths {
					if strings.TrimSpace(path) == "" {
						continue
					}
					if _, err := os.Stat(path); err == nil {
						if err := exec.Command("sudo", "rm", "-rf", path).Run(); err != nil {
							lastErr = fmt.Errorf("failed to remove share directory %s: %w", path, err)
							fmt.Fprintln(os.Stderr, lastErr)
						} else {
							fmt.Printf("Removed directory at %s\n", path)
						}
					}
				}
			}

			// Try to remove desktop entry if it exists
			desktopEntry := filepath.Join("/usr/local/share/applications", packageName+".desktop")
			if _, err := os.Stat(desktopEntry); err == nil {
				if err := exec.Command("sudo", "rm", "-f", desktopEntry).Run(); err != nil {
					lastErr = fmt.Errorf("failed to remove desktop entry %s: %w", desktopEntry, err)
					fmt.Fprintln(os.Stderr, lastErr)
				} else {
					fmt.Printf("Removed desktop entry at %s\n", desktopEntry)
				}
			}

			// Unregister from registry
			if err := registry.Unregister(packageName); err != nil {
				lastErr = err
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Printf("Unregistered package '%s' from registry\n", packageName)
			}
		}
		return lastErr
	},
}
