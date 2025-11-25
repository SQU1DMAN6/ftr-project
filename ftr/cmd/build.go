package cmd

import (
	"fmt"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	// No need to register here
}

var buildCmd = &cobra.Command{
	Use:   "build [file] [executable name]",
	Short: "Build an existing .fsdl file",
	Long: `Build an existing .fsdl file containing a main.py, main.go, or main.cpp, with an optional install.sh or Makefile into a computer-ready package.

	Example: ftr build myproject.fsdl myproject`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsdlFilePath := args[0]
		repoName := args[1]

		// Check if a temporary build directory is available
		tmpDir := "/tmp/fsdl"
		_, err := os.Stat(tmpDir)
		if err == nil {
			return fmt.Errorf("temporary directory '%s' already exists. Consider running 'ftr clear' or renaming the directory before proceeding", tmpDir)
		}

		// Create a temporary directory
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create temporary directory at '%s': %w", tmpDir, err)
		}
		defer os.RemoveAll(tmpDir) // Clean up after the operation

		// Open the .fsdl file to be extracted
		sourceFile, err := os.Open(fsdlFilePath)
		if err != nil {
			return fmt.Errorf("failed to open source file '%s': %w", fsdlFilePath, err)
		}
		defer sourceFile.Close()

		// Prepare the file to be copied to the temporary directory
		destinationFilePath := filepath.Join(tmpDir, filepath.Base(fsdlFilePath))
		destinationFile, err := os.Create(destinationFilePath)
		if err != nil {
			return fmt.Errorf("failed to create destination file '%s': %w", destinationFilePath, err)
		}
		defer destinationFile.Close()

		// Copy the contents of the source .fsdl file to the temporary directory
		_, err = io.Copy(destinationFile, sourceFile)
		if err != nil {
			return fmt.Errorf("failed to copy source file to temporary directory: %w", err)
		}
		fmt.Printf("Successfully copied '%s' to '%s'.\n", fsdlFilePath, destinationFilePath)

		// Change working directory to temporary directory for extraction
		if err := os.Chdir(tmpDir); err != nil {
			return fmt.Errorf("failed to change working directory to '%s': %w", tmpDir, err)
		}

		// Extract the contents of the .fsdl file
		if err := fsdl.Extract(destinationFilePath, tmpDir); err != nil {
			return fmt.Errorf("failed to extract .fsdl package: %w", err)
		}

		// Initialize the builder with repo name and working directory
		b := builder.New(repoName, tmpDir)

		// Detect and build the project
		binaryPath, err := b.DetectAndBuild()
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		// Install if a binary was produced
		if binaryPath != "" {
			if err := b.InstallBinary(binaryPath); err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}
		}

		// Successful build and installation
		fmt.Println("Build and installation completed successfully.")
		return nil
	},
}
