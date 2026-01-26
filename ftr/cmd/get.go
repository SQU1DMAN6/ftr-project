package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/boxlet"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"ftr/pkg/registry"
	"ftr/pkg/sqar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().Bool("no-unzip", false, "Skip extraction and installation")
	getCmd.Flags().BoolP("ask", "A", false, "Prompt to select which file to download from repository")
}

func normArch(a string) string {
	a = strings.TrimSpace(a)
	a = strings.ToLower(a)
	if a == "amd64" {
		return "x64"
	}
	return a
}

func normOS(o string) string {
	return strings.ToLower(strings.TrimSpace(o))
}

func matchesTarget(t string, local string) bool {
	if t == "" {
		return true
	}
	for _, tok := range strings.Split(t, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "all" || tok == local {
			return true
		}
	}
	return false
}

func extractSqar(sqarPath, destDir string) error {
	// Ensure destination exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	sqarTool := sqar.FindSqarTool()
	if sqarTool == "" {
		return fmt.Errorf("sqar tool not found on system")
	}

	cmd := exec.Command(sqarTool, "unpack", sqarPath, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

var getCmd = &cobra.Command{
	Use:   "get [user/repo]...",
	Short: "Download and install a repository",
	Long: `Download and install a repository package from the server.
The package will be downloaded as an FSDL file, extracted, and built if possible.

Example: ftr get user/myapp`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noUnzip, _ := cmd.Flags().GetBool("no-unzip")
		askFlag, _ := cmd.Flags().GetBool("ask")

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// determine local architecture/os once
		localArch := runtime.GOARCH
		if localArch == "amd64" {
			localArch = "x64"
		}
		localOS := runtime.GOOS

		var lastErr error
		for _, repoPath := range args {
			// Parse user/repo and optional @version (user/repo@1.2.3)
			rp := repoPath
			var version string
			if strings.Contains(rp, "@") {
				sp := strings.SplitN(rp, "@", 2)
				rp = sp[0]
				version = sp[1]
			}

			parts := strings.Split(rp, "/")
			if len(parts) != 2 {
				lastErr = fmt.Errorf("invalid repository path '%s'. Must be in format user/repo or user/repo@version", repoPath)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}
			repoName := parts[1]
			user := parts[0]

			// Use fixed extraction directory so users can inspect / control it
			tmpDir := "/tmp/fsdl"
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				lastErr = fmt.Errorf("failed to ensure /tmp/fsdl exists: %w", err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			// Download from server
			fmt.Printf("Fetching repo: %s\n", repoPath)

			// Try to fetch repository description to show to the user
			if matches, err := client.SearchRepos(repoName); err == nil {
				for _, m := range matches {
					if m["user"] == parts[0] && m["repo"] == repoName {
						desc := m["description"]
						if desc == "" {
							desc = "(no description)"
						}
						fmt.Printf("Description: %s\n", desc)
						break
					}
				}
			}

			fmt.Printf("Fetching package via API...\n")

			// Determine desired architecture and OS locally
			localArch = runtime.GOARCH
			if localArch == "amd64" {
				localArch = "x64"
			}
			localOS = runtime.GOOS

			// If -A flag used, let user pick a file from repository listing
			var chosenFile string
			if askFlag {
				files, err := client.ListRepoFiles(user, repoName)
				if err != nil {
					lastErr = fmt.Errorf("failed to list repo files for %s: %w", repoPath, err)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
				if len(files) == 0 {
					lastErr = fmt.Errorf("no files available in repository %s", repoPath)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
				fmt.Println("Available files:")
				for i, f := range files {
					name := ""
					if n, ok := f["name"].(string); ok {
						name = n
					} else if p, ok := f["path"].(string); ok {
						name = p
					}
					fmt.Printf("[%d] %s\n", i, name)
				}
				fmt.Printf("Choose file index: ")
				var idx int
				if _, err := fmt.Scanln(&idx); err != nil {
					lastErr = fmt.Errorf("invalid selection")
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
				if idx < 0 || idx >= len(files) {
					lastErr = fmt.Errorf("selection out of range")
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
				if n, ok := files[idx]["name"].(string); ok {
					chosenFile = n
				}
			}

			// helper to attempt download of a filename (sqar or fsdl)
			tryDownload := func(remote string) (string, error) {
				var dest string
				if strings.HasSuffix(remote, ".sqar") {
					dest = filepath.Join(tmpDir, remote)
				} else {
					dest = filepath.Join(tmpDir, filepath.Base(remote))
				}
				if err := client.DownloadAndVerify(user, repoName, remote, dest, nil); err != nil {
					return "", err
				}
				return dest, nil
			}

			var usedSqar bool
			var downloadedPath string

			// Check if SQAR is available
			sqarAvailable := sqar.FindSqarTool() != ""

			if chosenFile != "" {
				// user selected exact file
				p, err := tryDownload(chosenFile)
				if err != nil {
					lastErr = fmt.Errorf("download failed for %s: %w", repoPath, err)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
				downloadedPath = p
				usedSqar = strings.HasSuffix(chosenFile, ".sqar")
			} else {
				candidates := []string{}
				if version != "" {
					if sqarAvailable {
						// prefer exact arch/os first (SQAR)
						candidates = append(candidates, fmt.Sprintf("%s-%s-%s-%s.sqar", repoName, version, localArch, localOS))
						// prefer all arch/os second (SQAR)
						candidates = append(candidates, fmt.Sprintf("%s-%s-all-%s.sqar", repoName, version, localOS))
						candidates = append(candidates, fmt.Sprintf("%s-%s-%s-all.sqar", repoName, version, localArch))
						candidates = append(candidates, fmt.Sprintf("%s-%s-all-all.sqar", repoName, version))
						// fallback (SQAR)
						candidates = append(candidates, fmt.Sprintf("%s-%s.sqar", repoName, version))
					}
					// FSDL fallback
					candidates = append(candidates, fmt.Sprintf("%s-%s-%s-%s.fsdl", repoName, version, localArch, localOS))
					candidates = append(candidates, fmt.Sprintf("%s-%s.fsdl", repoName, version))
				} else {
					files, err := client.ListRepoFiles(user, repoName)
					if err == nil {
						var sqarMatches []string
						var fsdlMatches []string
						var platformMatches []string // files matching our exact platform

						for _, f := range files {
							name := ""
							if n, ok := f["name"].(string); ok {
								name = n
							} else if p, ok := f["path"].(string); ok {
								name = p
							}
							if name == "" {
								continue
							}
							lname := strings.ToLower(name)
							if strings.Contains(lname, strings.ToLower(repoName)) {
								if strings.HasSuffix(lname, ".sqar") {
									sqarMatches = append(sqarMatches, name)
									// check if this matches our platform
									if strings.Contains(lname, localArch) && strings.Contains(lname, localOS) {
										platformMatches = append(platformMatches, name)
									}
								} else if strings.HasSuffix(lname, ".fsdl") {
									fsdlMatches = append(fsdlMatches, name)
									// check if this matches our platform
									if strings.Contains(lname, localArch) && strings.Contains(lname, localOS) {
										platformMatches = append(platformMatches, name)
									}
								}
							}
						}

						// If platform-specific files exist, prefer them first
						if len(platformMatches) > 0 {
							// Sort to get highest version first
							type fileVer struct{ name, ver string }
							var versionedMatches []fileVer
							for _, n := range platformMatches {
								if strings.HasPrefix(n, repoName+"-") {
									rest := strings.TrimPrefix(n, repoName+"-")
									parts := strings.Split(rest, "-")
									if len(parts) > 0 {
										ver := parts[0]
										versionedMatches = append(versionedMatches, fileVer{name: n, ver: ver})
									}
								}
							}
							if len(versionedMatches) > 0 {
								sort.Slice(versionedMatches, func(i, j int) bool {
									return compareVersions(versionedMatches[i].ver, versionedMatches[j].ver) > 0
								})
								for _, fv := range versionedMatches {
									candidates = append(candidates, fv.name)
								}
							}
							for _, n := range platformMatches {
								if !strings.HasPrefix(n, repoName+"-") {
									candidates = append(candidates, n)
								}
							}
						}

						// If SQAR is available, prefer versioned sqar (highest version) and matching arch/os
						if sqarAvailable && len(sqarMatches) > 0 {
							type fileVer struct{ name, ver string }
							var found []fileVer
							for _, n := range sqarMatches {
								if strings.HasPrefix(n, repoName+"-") {
									rest := strings.TrimPrefix(n, repoName+"-")
									parts := strings.Split(rest, "-")
									if len(parts) > 0 {
										ver := parts[0]
										found = append(found, fileVer{name: n, ver: ver})
									}
								}
							}
							if len(found) > 0 {
								sort.Slice(found, func(i, j int) bool {
									return compareVersions(found[i].ver, found[j].ver) > 0
								})
								// Prioritize: exact arch/os match > any arch + matching os > all + any arch
								chosen := ""
								chosen_score := -1
								for _, f := range found {
									lname := strings.ToLower(f.name)
									larch := strings.ToLower(localArch)
									los := strings.ToLower(localOS)

									// Score: 3=exact match, 2=matching OS only, 1=anything else
									score := 0
									hasExactArch := strings.Contains(lname, larch)
									hasExactOS := strings.Contains(lname, los)
									hasAllArch := strings.Contains(lname, "all") && !strings.Contains(lname, larch)

									if hasExactArch && hasExactOS {
										score = 3
									} else if hasExactOS || (hasAllArch && hasExactOS) {
										score = 2
									}

									if score > chosen_score {
										chosen = f.name
										chosen_score = score
									}
								}
								if chosen == "" {
									// no match at all; pick highest versioned file
									chosen = found[0].name
								}
								// Use the single chosen versioned file as the primary candidate
								candidates = append(candidates, chosen)
							} else {
								// fallback: try all sqar files, then fsdl
								for _, n := range sqarMatches {
									candidates = append(candidates, n)
								}
								for _, n := range fsdlMatches {
									candidates = append(candidates, n)
								}
							}
						} else {
							// SQAR not available, prefer FSDL first
							for _, n := range fsdlMatches {
								candidates = append(candidates, n)
							}
							// then fall back to SQAR if available
							for _, n := range sqarMatches {
								candidates = append(candidates, n)
							}
						}
					}
					// final fallback
					if sqarAvailable {
						candidates = append(candidates, fmt.Sprintf("%s.sqar", repoName))
					}
					candidates = append(candidates, fmt.Sprintf("%s.fsdl", repoName))
				}

				attempted := []string{}
				var dlErr error
				for _, c := range candidates {
					attempted = append(attempted, c)
					downloadedPath, dlErr = tryDownload(c)
					if dlErr == nil {
						usedSqar = strings.HasSuffix(c, ".sqar")
						break
					}
				}
				if downloadedPath == "" {
					lastErr = fmt.Errorf("download failed for %s; no files to download", repoPath)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
			}

			// Note: platform mismatch warning is shown later after extraction
			// when we have more complete information from BUILD/Meta.config

			if noUnzip {
				fmt.Println("--no-unzip used. Skipping extraction and install.")
				continue
			}

			// Extract the package based on type
			if usedSqar {
				if err := extractSqar(downloadedPath, tmpDir); err != nil {
					lastErr = fmt.Errorf("failed to extract sqar package for %s: %w", repoPath, err)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
			} else {
				if err := fsdl.Extract(downloadedPath, tmpDir); err != nil {
					lastErr = fmt.Errorf("failed to extract package for %s: %w", repoPath, err)
					fmt.Fprintln(os.Stderr, lastErr)
					continue
				}
			}

			// After extraction: determine target arch/os from filename and BUILD/Meta.config
			filename := filepath.Base(downloadedPath)
			ext := filepath.Ext(filename)
			base := strings.TrimSuffix(filename, ext)
			fileTargetArch := ""
			fileTargetOS := ""
			if strings.HasPrefix(base, repoName+"-") {
				rest := strings.TrimPrefix(base, repoName+"-")
				parts := strings.Split(rest, "-")
				if len(parts) >= 2 {
					fileTargetArch = parts[len(parts)-2]
					fileTargetOS = parts[len(parts)-1]
				}
			}

			// Read BUILD/Meta.config if present
			meta, merr := boxlet.ReadMeta(tmpDir)
			metaArch := ""
			metaOS := ""
			if merr == nil && meta != nil {
				if v, ok := meta["TARGET_ARCHITECTURE"]; ok {
					metaArch = v
				}
				if v, ok := meta["TARGET_OS"]; ok {
					metaOS = v
				}
			}

			// choose authoritative target values: prefer meta, fall back to filename
			targetArch := fileTargetArch
			if metaArch != "" {
				targetArch = metaArch
			}
			targetOS := fileTargetOS
			if metaOS != "" {
				targetOS = metaOS
			}

			// Get local platform (already normalized from earlier)
			la := normArch(localArch)
			lo := normOS(localOS)

			// Normalize target values
			ta := normArch(targetArch)
			to := normOS(targetOS)

			// Check if target platform matches local platform
			okArch := matchesTarget(ta, la)
			okOS := matchesTarget(to, lo)

			if !okArch || !okOS {
				fmt.Printf("Warning: package targets %s/%s which does not match your system %s/%s\n", ta, to, la, lo)
				fmt.Printf("Proceed? [y/N] ")
				var ans string
				if _, err := fmt.Scanln(&ans); err != nil || (strings.ToLower(strings.TrimSpace(ans)) != "y") {
					fmt.Println("Skipping installation for", repoPath)
					continue
				}
			}

			// Initialize builder
			// Determine workdir: sometimes archives create a top-level folder; find install.sh or choose single top-level dir
			workDir := tmpDir
			_ = filepath.WalkDir(tmpDir, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				base := filepath.Base(p)
				if base == "install.sh" || base == "Makefile" || strings.HasPrefix(base, "main.") || base == "BUILD" || base == "Meta.config" {
					workDir = filepath.Dir(p)
					return filepath.SkipDir
				}
				return nil
			})
			// If tmpDir contains a single directory and nothing else, prefer that
			entries, _ := os.ReadDir(tmpDir)
			if len(entries) == 1 && entries[0].IsDir() {
				workDir = filepath.Join(tmpDir, entries[0].Name())
			}
			b := builder.New(repoName, workDir)

			// Detect and build
			binaryPath, err := b.DetectAndBuild()
			if err != nil {
				lastErr = fmt.Errorf("build failed for %s: %w", repoPath, err)
				fmt.Fprintln(os.Stderr, lastErr)
				continue
			}

			// Install if binary was produced
			var installedBinaryPath string
			if binaryPath != "" {
				// Check if this is a path from install.sh (absolute path in /usr/local/bin)
				// vs a path from DetectAndBuild that needs to be installed
				if strings.HasPrefix(binaryPath, "/usr/local/bin/") {
					// This binary was installed by install.sh, just register it
					installedBinaryPath = binaryPath
				} else {
					// This is a binary from the build system, needs InstallBinary processing
					var err error
					var installedSharePath string
					installedBinaryPath, installedSharePath, err = b.InstallBinary(binaryPath)
					_ = installedSharePath // Not used in new approach
					if err != nil {
						lastErr = fmt.Errorf("installation failed for %s: %w", repoPath, err)
						fmt.Fprintln(os.Stderr, lastErr)
						continue
					}
				}
			}

			// Determine installed version: prefer BUILD/Meta.config VERSION, then requested version
			installedVersion := version
			if meta != nil {
				if v, ok := meta["VERSION"]; ok && strings.TrimSpace(v) != "" {
					installedVersion = strings.TrimSpace(v)
				}
			}
			// Register package in registry (without install paths - remove will use remove.sh or defaults)
			regInfo := registry.PackageInfo{
				Name:    repoName,
				Version: installedVersion,
				Source:  repoPath,
				// Note: InstallPath is NOT recorded - removal uses remove.sh or standard paths
				BinaryPath: installedBinaryPath,
			}
			if err := registry.Register(regInfo); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to register package in registry: %v\n", err)
			}

			fmt.Println("Done.")
		}

		// compareVersions is implemented in cmd/list.go and reused here.
		return lastErr
	},
}
