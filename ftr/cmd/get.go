package cmd

import (
	"errors"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"ftr/pkg/registry"
	"ftr/pkg/sqar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().Bool("no-unzip", false, "Skip extraction and installation")
}

var getCmd = &cobra.Command{
	Use:   "get [user/repo]",
	Short: "Download and install a repository",
	Long: `Download and install a repository package from the server.
The package will be downloaded as an FSDL file, extracted, and built if possible.

Example: ftr get user/myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		noUnzip, _ := cmd.Flags().GetBool("no-unzip")

		// Split user/repo
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository path. Must be in format user/repo")
		}
		repoName := parts[1]

		// Create temporary directory
		tmpDir := "/tmp/fsdl"
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}

		fsdlFile := filepath.Join(tmpDir, repoName+".fsdl")

		// Download from server
		fmt.Printf("Fetching repo: %s\n", repoPath)

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

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
		// Try SQAR first (default), then fall back to FSDL
		sqarFile := filepath.Join(tmpDir, repoName+".sqar")
		fsdlFile = filepath.Join(tmpDir, repoName+".fsdl")

		downloadErr := client.DownloadAndVerify(parts[0], repoName, repoName+".sqar", sqarFile, nil)
		usedSqar := true
		if downloadErr != nil {
			// fall back to fsdl
			if err := client.DownloadAndVerify(parts[0], repoName, repoName+".fsdl", fsdlFile, nil); err != nil {
				return fmt.Errorf("download failed (tried .sqar and .fsdl): %w / %v", err, downloadErr)
			}
			usedSqar = false
		}

		fmt.Println()

		if noUnzip {
			fmt.Println("--no-unzip used. Skipping extraction and install.")
			return nil
		}

		// Extract the package based on type
		if usedSqar {
			if err := extractSqar(sqarFile, tmpDir); err != nil {
				return fmt.Errorf("failed to extract sqar package: %w", err)
			}
		} else {
			if err := fsdl.Extract(fsdlFile, tmpDir); err != nil {
				return fmt.Errorf("failed to extract package: %w", err)
			}
		}

		// Initialize builder
		b := builder.New(repoName, tmpDir)

		// Detect and build
		binaryPath, err := b.DetectAndBuild()
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		// Install if binary was produced
		if binaryPath != "" {
			if err := b.InstallBinary(binaryPath); err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}
		}

		// Register package in registry
		regInfo := registry.PackageInfo{
			Name:        repoName,
			Version:     "",
			Source:      repoPath,
			InstallPath: "/usr/local/share/" + repoName,
			BinaryPath:  "/usr/local/bin/" + repoName,
		}
		_ = registry.Register(regInfo)

		fmt.Println("Done.")
		return nil
	},
}

func extractSqar(sqarPath, destDir string) error {
	sqarTool := sqar.FindSqarTool()
	if sqarTool != "" {
		cmd := exec.Command(sqarTool, "unpack", sqarPath, destDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return err
		}
	}
	return errors.New("failed to find SQAR archiving utility. Consider getting SQAR first.")
}
