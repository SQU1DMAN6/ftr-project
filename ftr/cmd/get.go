package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"io"
	"os"
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
		user, repoName := parts[0], parts[1]

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

		downloadPath := fmt.Sprintf("%s/%s/%s/%s.fsdl", api.RepoURL, user, repoName, repoName)

		fmt.Printf("Trying %s ...\n", downloadPath)
		reader, err := client.DownloadFile(downloadPath, repoName+".fsdl")
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer reader.Close()

		// Save to temp file
		f, err := os.Create(fsdlFile)
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer f.Close()

		if _, err := io.Copy(f, reader); err != nil {
			return fmt.Errorf("failed to save downloaded file: %w", err)
		}

		if noUnzip {
			fmt.Println("--no-unzip used. Skipping extraction and install.")
			return nil
		}

		// Extract the package
		if err := fsdl.Extract(fsdlFile, tmpDir); err != nil {
			return fmt.Errorf("failed to extract package: %w", err)
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

		fmt.Println("Done.")
		return nil
	},
}
