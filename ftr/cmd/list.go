package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/registry"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed or upgradeable packages",
	Long:  "List installed packages (-I), upgradeable packages (-U), or just names (-q).",
	RunE: func(cmd *cobra.Command, args []string) error {
		installedOnly, _ := cmd.Flags().GetBool("installed")
		upgradeableOnly, _ := cmd.Flags().GetBool("upgradeable")
		quiet, _ := cmd.Flags().GetBool("quiet")

		// default to installed if no flags
		if !installedOnly && !upgradeableOnly && !quiet {
			installedOnly = true
		}

		pkgs, err := registry.List()
		if err != nil {
			return fmt.Errorf("failed to load registry: %w", err)
		}

		if quiet && !upgradeableOnly {
			for _, p := range pkgs {
				fmt.Println(p.Name)
			}
			return nil
		}

		client, _ := api.NewClient()

		if installedOnly {
			for _, p := range pkgs {
				ver := p.Version
				if ver == "" {
					ver = "(unknown)"
				}
				if quiet {
					fmt.Println(p.Name)
				} else {
					fmt.Printf("%s %s (%s)\n", p.Name, ver, p.Source)
				}
			}
		}

		if upgradeableOnly {
			for _, p := range pkgs {
				if p.Source == "" {
					continue
				}
				parts := strings.Split(p.Source, "/")
				if len(parts) != 2 {
					continue
				}
				user := parts[0]
				
				// Extract repo name, removing any @version suffix
				repo := parts[1]
				if idx := strings.Index(repo, "@"); idx != -1 {
					repo = repo[:idx]
				}
				
				// Skip packages with empty versions
				if p.Version == "" {
					continue
				}
				
				// Get list of files available in the repository
				files, err := client.ListRepoFiles(user, repo)
				if err != nil {
					// ignore errors and continue to next package
					continue
				}
				
				// Find the latest version from filenames
				remoteVer := ""
				for _, file := range files {
					// The API returns "path" not "name"
					fileName, ok := file["path"].(string)
					if !ok {
						continue
					}
					
					// Try to extract version from filename
					// Expected format: packagename-version.fsdl or packagename-version.sqar
					v := extractVersionFromFilename(fileName, p.Name)
					if v != "" && compareVersions(remoteVer, v) < 0 {
						remoteVer = v
					}
				}
				
				if remoteVer == "" {
					continue
				}
				cmp := compareVersions(p.Version, remoteVer)
				if cmp < 0 {
					if quiet {
						fmt.Println(p.Name)
					} else {
						fmt.Printf("%s %s -> %s (%s)\n", p.Name, p.Version, remoteVer, p.Source)
					}
				}
			}
		}

		return nil
	},
}

func init() {
	listCmd.Flags().BoolP("installed", "I", false, "List installed packages with versions")
	listCmd.Flags().BoolP("upgradeable", "U", false, "List upgradeable packages with remote versions")
	listCmd.Flags().BoolP("quiet", "q", false, "Quiet: list only package names")
	rootCmd.AddCommand(listCmd)
}

// compareVersions compares semantic version-like strings (basic numeric parts).
// returns -1 if a<b, 0 if equal, 1 if a>b
func compareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai := 0
		bi := 0
		if i < len(as) {
			ai, _ = strconv.Atoi(strings.TrimFunc(as[i], func(r rune) bool { return r < '0' || r > '9' }))
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(strings.TrimFunc(bs[i], func(r rune) bool { return r < '0' || r > '9' }))
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

// extractVersionFromFilename extracts version from filenames like "package-name-1.0.0.fsdl"
// It looks for the pattern: packagename-version.extension
func extractVersionFromFilename(fileName, packageName string) string {
	// Remove extension
	withoutExt := strings.TrimSuffix(strings.TrimSuffix(fileName, ".fsdl"), ".sqar")
	
	// Look for pattern: packagename-version
	prefix := packageName + "-"
	if strings.HasPrefix(withoutExt, prefix) {
		version := strings.TrimPrefix(withoutExt, prefix)
		// Validate it looks like a version (starts with a digit)
		if len(version) > 0 && version[0] >= '0' && version[0] <= '9' {
			return version
		}
	}
	
	return ""
}
