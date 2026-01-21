package cmd

import (
	"encoding/json"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/boxlet"
	"ftr/pkg/builder"
	"ftr/pkg/registry"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all installed packages",
	Long:  "Checks installed packages and attempts to fetch and install their latest versions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgs, err := registry.List()
		if err != nil {
			return fmt.Errorf("failed to list installed packages: %w", err)
		}
		var lastErr error
		client, _ := api.NewClient()

		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".local", "share", "ftr", "cache")

		localArch := runtime.GOARCH
		if localArch == "amd64" {
			localArch = "x64"
		}
		localOS := runtime.GOOS

		for _, p := range pkgs {
			if p.Source == "" {
				fmt.Printf("Skipping %s (no source metadata)\n", p.Name)
				continue
			}
			fmt.Printf("Updating %s...\n", p.Name)
			parts := strings.Split(p.Source, "/")
			if len(parts) != 2 {
				fmt.Printf("Invalid source for %s: %s\n", p.Name, p.Source)
				continue
			}
			user := parts[0]
			repo := parts[1]

			// Try to use cache to select candidate
			cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s_%s.files.json", user, repo))
			chosen := ""
			if data, err := os.ReadFile(cacheFile); err == nil {
				var files []map[string]interface{}
				if err := json.Unmarshal(data, &files); err == nil {
					var sqarMatches []string
					for _, f := range files {
						name := ""
						if n, ok := f["name"].(string); ok {
							name = n
						} else if pth, ok := f["path"].(string); ok {
							name = pth
						}
						if name == "" {
							continue
						}
						lname := strings.ToLower(name)
						if strings.Contains(lname, strings.ToLower(repo)) && strings.HasSuffix(lname, ".sqar") {
							sqarMatches = append(sqarMatches, name)
						}
					}
					// pick best versioned file
					type fileVer struct{ name, ver string }
					var found []fileVer
					for _, n := range sqarMatches {
						if strings.HasPrefix(n, repo+"-") {
							rest := strings.TrimPrefix(n, repo+"-")
							parts := strings.Split(rest, "-")
							if len(parts) > 0 {
								ver := parts[0]
								found = append(found, fileVer{name: n, ver: ver})
							}
						}
					}
					if len(found) > 0 {
						sort.Slice(found, func(i, j int) bool { return compareVersions(found[i].ver, found[j].ver) > 0 })
						// prefer arch/os match
						for _, f := range found {
							lname := strings.ToLower(f.name)
							if strings.Contains(lname, strings.ToLower(localArch)) && strings.Contains(lname, strings.ToLower(localOS)) {
								chosen = f.name
								break
							}
						}
						if chosen == "" {
							chosen = found[0].name
						}
					}
				}
			}

			if chosen == "" {
				// Fallback: call get to perform selection (will hit server)
				if err := getCmd.RunE(getCmd, []string{p.Source}); err != nil {
					lastErr = err
					fmt.Printf("Failed to update %s: %v\n", p.Name, err)
				}
				continue
			}

			// Download chosen file
			dest := filepath.Join("/tmp/fsdl", chosen)
			if err := client.DownloadAndVerify(user, repo, chosen, dest, nil); err != nil {
				lastErr = fmt.Errorf("failed to download %s for %s: %w", chosen, p.Name, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			// Extract and install similar to get
			if err := extractSqar(dest, "/tmp/fsdl"); err != nil {
				lastErr = fmt.Errorf("failed to extract package for %s: %w", p.Name, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			// Determine workdir
			workDir := "/tmp/fsdl"
			_ = filepath.WalkDir(workDir, func(pth string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				base := filepath.Base(pth)
				if base == "install.sh" || base == "Makefile" || strings.HasPrefix(base, "main.") || base == "BUILD" || base == "Meta.config" {
					workDir = filepath.Dir(pth)
					return filepath.SkipDir
				}
				return nil
			})

			b := builder.New(repo, workDir)
			binaryPath, err := b.DetectAndBuild()
			if err != nil {
				lastErr = fmt.Errorf("build failed for %s: %w", p.Name, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}
			if binaryPath != "" {
				if err := b.InstallBinary(binaryPath); err != nil {
					lastErr = fmt.Errorf("installation failed for %s: %w", p.Name, err)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
			}

			// Read version from META if present
			meta, _ := boxlet.ReadMeta("/tmp/fsdl")
			if meta != nil {
				if v, ok := meta["VERSION"]; ok && strings.TrimSpace(v) != "" {
					p.Version = strings.TrimSpace(v)
					_ = registry.Register(p)
				}
			}
		}
		return lastErr
	},
}
