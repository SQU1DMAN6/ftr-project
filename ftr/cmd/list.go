package cmd

import (
	"bufio"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/registry"
	"os"
	"path/filepath"
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
				user, repo := parts[0], parts[1]
				
				// Skip packages with empty versions
				if p.Version == "" {
					continue
				}
				
				tmp := filepath.Join(os.TempDir(), repo+".meta.tmp")
				if err := client.DownloadAndVerify(user, repo, "BUILD/Meta.config", tmp, nil); err != nil {
					// ignore errors fetching metadata and continue to next package
					continue
				}
				f, err := os.Open(tmp)
				if err != nil {
					continue
				}
				scanner := bufio.NewScanner(f)
				remoteVer := ""
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if strings.HasPrefix(line, "VERSION=") {
						remoteVer = strings.TrimPrefix(line, "VERSION=")
						remoteVer = strings.TrimSpace(remoteVer)
						break
					}
				}
				f.Close()
				os.Remove(tmp)
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
