package cmd

import (
	"bufio"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"ftr/pkg/registry"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [package]...",
	Short: "Upgrade installed packages to the latest version",
	Long: `Upgrade installed packages to the latest version available in their source repositories.
If no package names are specified, all upgradeable packages will be listed for upgrade.

Example: ftr upgrade myapp
Example: ftr upgrade  # upgrades all upgradeable packages`,
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")

		pkgs, err := registry.List()
		if err != nil {
			return fmt.Errorf("failed to load registry: %w", err)
		}

		if len(pkgs) == 0 {
			fmt.Println("No packages installed.")
			return nil
		}

		client, _ := api.NewClient()

		// Determine which packages need upgrading
		var upgradeable []upgradeInfo
		filterByName := len(args) > 0

		for _, p := range pkgs {
			// If specific packages requested, skip those not in the list
			if filterByName {
				found := false
				for _, arg := range args {
					if p.Name == arg {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

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

			remoteVer, err := fetchRemoteVersion(client, user, repo, p.Name)
			if err != nil {
				// silently skip packages where we can't fetch remote version
				continue
			}

			if remoteVer == "" {
				continue
			}

			// Check if upgrade is needed
			cmp := compareVersions(p.Version, remoteVer)
			if cmp < 0 {
				upgradeable = append(upgradeable, upgradeInfo{
					Package:   p,
					RemoteVer: remoteVer,
					User:      user,
					Repo:      repo,
				})
			}
		}

		if len(upgradeable) == 0 {
			if filterByName {
				fmt.Println("No upgradeable packages found matching the specified names.")
			} else {
				fmt.Println("All packages are up to date.")
			}
			return nil
		}

		// Display upgradeable packages
		fmt.Println("The following packages can be upgraded:")
		for i, upg := range upgradeable {
			fmt.Printf("%d. %s %s -> %s (%s/%s)\n", i+1, upg.Package.Name, upg.Package.Version, upg.RemoteVer, upg.User, upg.Repo)
		}

		if !yes {
			fmt.Print("\nDo you want to upgrade these packages? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadByte()
			if err != nil {
				return err
			}
			if response != 'y' {
				fmt.Println("Upgrade cancelled.")
				return nil
			}
		}

		// Perform upgrades
		fmt.Println("\nUpgrading packages...")
		var failedUpgrades []string

		for _, upg := range upgradeable {
			fmt.Printf("Upgrading %s from %s to %s...\n", upg.Package.Name, upg.Package.Version, upg.RemoteVer)

			// Use the get command logic to download and install
			repoPath := fmt.Sprintf("%s/%s@%s", upg.User, upg.Repo, upg.RemoteVer)

			// Parse and download the package
			tmpDir := "/tmp/fsdl"
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create temp directory: %v\n", err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			// Try to find and download the package file
			files, err := client.ListRepoFiles(upg.User, upg.Repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list files for %s: %v\n", repoPath, err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			// Find FSDL or SQAR file for this version
			packageFile := ""
			for _, f := range files {
				if fName, ok := f["path"].(string); ok {
					// Look for files matching the version and containing our architecture/os
					if strings.Contains(fName, upg.RemoteVer) && (strings.HasSuffix(fName, ".fsdl") || strings.HasSuffix(fName, ".sqar")) {
						packageFile = fName
						break
					}
				}
			}

			if packageFile == "" {
				// Fallback: try to download as generic file
				if len(files) > 0 {
					if fName, ok := files[0]["name"].(string); ok {
						if strings.HasSuffix(fName, ".fsdl") || strings.HasSuffix(fName, ".sqar") {
							packageFile = fName
						}
					}
				}
			}

			if packageFile == "" {
				fmt.Fprintf(os.Stderr, "No suitable package file found for %s\n", repoPath)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			// Download the package
			localFilePath := filepath.Join(tmpDir, packageFile)
			fmt.Printf("Downloading %s...\n", packageFile)

			if err := client.DownloadAndVerify(upg.User, upg.Repo, packageFile, localFilePath, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", packageFile, err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create temporary build directory: %v\n", err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			// Extract based on file type
			if strings.HasSuffix(packageFile, ".sqar") {
				if err := extractSqar(localFilePath, tmpDir); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to extract SQAR: %v\n", err)
					failedUpgrades = append(failedUpgrades, upg.Package.Name)
					continue
				}
			} else if strings.HasSuffix(packageFile, ".fsdl") {
				if err := fsdl.Extract(localFilePath, tmpDir); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to extract FSDL: %v\n", err)
					failedUpgrades = append(failedUpgrades, upg.Package.Name)
					continue
				}
			}

			// Build if it's a Go application or shell script
			b := builder.New(upg.Package.Name, tmpDir)
			binaryPath, err := b.DetectAndBuild()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Build failed for %s: %v\n", upg.Package.Name, err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			// Install the binary if it was produced
			if binaryPath != "" {
				if err := b.InstallBinary(binaryPath); err != nil {
					fmt.Fprintf(os.Stderr, "Installation failed for %s: %v\n", upg.Package.Name, err)
					failedUpgrades = append(failedUpgrades, upg.Package.Name)
					continue
				}
			}

			// Update registry with new version
			upg.Package.Version = upg.RemoteVer
			if err := registry.Register(upg.Package); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to update registry for %s: %v\n", upg.Package.Name, err)
				failedUpgrades = append(failedUpgrades, upg.Package.Name)
				continue
			}

			fmt.Println("Cleaning temporary build directory...")
			os.RemoveAll(tmpDir)

			fmt.Printf("Successfully upgraded %s to %s\n", upg.Package.Name, upg.RemoteVer)
		}

		// Summary
		if len(failedUpgrades) > 0 {
			fmt.Printf("\nUpgrade completed with %d failure(s):\n", len(failedUpgrades))
			for _, name := range failedUpgrades {
				fmt.Printf("  - %s\n", name)
			}
			return fmt.Errorf("some packages failed to upgrade")
		}

		fmt.Println("\nAll packages upgraded successfully!")
		return nil
	},
}

// upgradeInfo represents a package that can be upgraded
type upgradeInfo struct {
	Package   registry.PackageInfo
	RemoteVer string
	User      string
	Repo      string
}

// fetchRemoteVersion fetches the remote version by listing repository files and extracting versions from filenames
func fetchRemoteVersion(client *api.Client, user, repo, packageName string) (string, error) {
	files, err := client.ListRepoFiles(user, repo)
	if err != nil {
		return "", err
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
		v := extractVersionFromFilename(fileName, packageName)
		if v != "" && compareVersions(remoteVer, v) < 0 {
			remoteVer = v
		}
	}

	return remoteVer, nil
}

func init() {
	upgradeCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(upgradeCmd)
}
