package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ftr/pkg/registry"
	"ftr/pkg/safety"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo]...",
	Short: "Remove an installed package",
	Long: `Remove an installed package from the system.
If no remove.sh exists, removes standard installation directories.
Before removing, scans for dangerous patterns in remove.sh and shows warnings.

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

			// Look up the package in the registry (for version info and confirmation)
			pkgInfo, err := registry.Find(packageName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: package '%s' not found in registry (but may still be installed)\n", packageName)
				// Continue anyway - we can still try to remove standard paths
			}

			// Check for remove.sh script
			removeScriptPath := filepath.Join("/usr/local/share", packageName, "remove.sh")
			if _, err := os.Stat(removeScriptPath); err == nil {
				// Found remove.sh - scan for dangerous patterns first
				fmt.Printf("Found remove.sh for '%s'. Scanning for dangerous patterns...\n\n", packageName)

				patterns, err := safety.ScanFileForDangerousPatterns(removeScriptPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to scan remove.sh: %v\n", err)
				}

				if len(patterns) > 0 {
					fmt.Printf("Found %d potentially dangerous pattern(s) in remove.sh:\n\n", len(patterns))
					for _, result := range patterns {
						formatted, _ := safety.FormatResultWithContext(removeScriptPath, result, 2)
						fmt.Println(formatted)
					}

					// Check severity - block if critical patterns found
					hasCritical := false
					for _, result := range patterns {
						if result.Pattern.Severity == "critical" {
							hasCritical = true
							break
						}
					}

					if hasCritical {
						fmt.Printf("\nCRITICAL: This script contains dangerous patterns that could damage your system.\n")
						fmt.Printf("Proceed with removal? [y/N] ")
						var ans string
						if _, err := fmt.Scanln(&ans); err != nil || (strings.ToLower(strings.TrimSpace(ans)) != "y") {
							fmt.Printf("Removal cancelled.\n")
							continue
						}
					}
				}

				// Execute remove.sh
				fmt.Printf("Executing remove.sh for '%s'...\n", packageName)
				if err := exec.Command("sudo", "sh", removeScriptPath).Run(); err != nil {
					lastErr = fmt.Errorf("remove.sh execution failed for %s: %w", packageName, err)
					fmt.Fprintln(os.Stderr, lastErr)
				} else {
					fmt.Printf("Successfully executed remove.sh\n")
				}
			} else {
				// No remove.sh - use standard removal paths
				fmt.Printf("No remove.sh found. Removing standard installation directories for '%s'...\n", packageName)

				// Remove the binary if it exists
				if pkgInfo != nil && pkgInfo.BinaryPath != "" {
					if _, err := os.Stat(pkgInfo.BinaryPath); err == nil {
						if err := exec.Command("sudo", "rm", "-f", pkgInfo.BinaryPath).Run(); err != nil {
							lastErr = fmt.Errorf("failed to remove binary %s: %w", pkgInfo.BinaryPath, err)
							fmt.Fprintln(os.Stderr, lastErr)
						} else {
							fmt.Printf("Removed binary at %s\n", pkgInfo.BinaryPath)
						}
					}
				}

				// Remove standard share directory
				shareDir := filepath.Join("/usr/local/share", packageName)
				if _, err := os.Stat(shareDir); err == nil {
					if err := exec.Command("sudo", "rm", "-rf", shareDir).Run(); err != nil {
						lastErr = fmt.Errorf("failed to remove share directory %s: %w", shareDir, err)
						fmt.Fprintln(os.Stderr, lastErr)
					} else {
						fmt.Printf("Removed directory at %s\n", shareDir)
					}
				}

				desktopEntry := filepath.Join("/usr/share/applications", packageName+".desktop")
				if _, err := os.Stat(desktopEntry); err == nil {
					if err := exec.Command("sudo", "rm", "-f", desktopEntry).Run(); err != nil {
						lastErr = fmt.Errorf("failed to remove desktop entry %s: %w", desktopEntry, err)
						fmt.Fprintln(os.Stderr, lastErr)
					} else {
						fmt.Printf("Removed desktop entry at %s\n", desktopEntry)
					}
				}
			}

			// Unregister from registry
			if pkgInfo != nil {
				if err := registry.Unregister(packageName); err != nil {
					lastErr = err
					fmt.Fprintln(os.Stderr, err)
				} else {
					fmt.Printf("Unregistered package '%s' from registry\n", packageName)
				}
			}
		}
		return lastErr
	},
}
