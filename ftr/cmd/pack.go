package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	// No need to register here
}

var packCmd = &cobra.Command{
	Use:   "pack [directory] [reponame]",
	Short: "Pack a directory into an .fsdl file",
	Long: `Pack a project directory containing a main.py, main.go, or main.cpp with an optional install.sh or Makefile into an .fsdl ready to upload to an InkDrop repository using 'ftr up'.

	Example: ftr pack myproject/ myproject`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		directoryPath := args[0]
		repoName := args[1]

		// Check if the directory exists and is valid
		info, err := os.Stat(directoryPath)
		if err != nil {
			return fmt.Errorf("failed to access project directory '%s': %w", directoryPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("'%s' is not a valid directory", directoryPath)
		}

		// Create the .fsdl file (zip archive)
		fsdlFileName := fmt.Sprintf("%s.fsdl", repoName)
		fsdlFile, err := os.Create(fsdlFileName)
		if err != nil {
			return fmt.Errorf("failed to create .fsdl file '%s': %w", fsdlFileName, err)
		}
		defer fsdlFile.Close()

		zipWriter := zip.NewWriter(fsdlFile)
		defer zipWriter.Close()

		// Walk through the directory and add all files (skip base dir itself and .fsdl files)
		err = filepath.WalkDir(directoryPath, func(filePath string, dirEntry os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip the base directory itself
			if filePath == directoryPath {
				return nil
			}

			// Skip any .fsdl file (including the one we're currently writing)
			if filepath.Ext(filePath) == ".fsdl" {
				return nil
			}

			// Get the relative path
			relPath, err := filepath.Rel(directoryPath, filePath)
			if err != nil {
				return fmt.Errorf("failed to get relative path for '%s': %w", filePath, err)
			}

			// If it's a directory, just ensure it's represented in the zip
			if dirEntry.IsDir() {
				_, err = zipWriter.Create(relPath + "/")
				return err
			}

			// Open the file
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file '%s': %w", filePath, err)
			}
			defer file.Close()

			// Get file info for the current file
			fileInfo, err := file.Stat()
			if err != nil {
				return fmt.Errorf("failed to get file info for '%s': %w", filePath, err)
			}

			// Create a header and writer for the file in the zip
			zipHeader, err := zip.FileInfoHeader(fileInfo)
			if err != nil {
				return fmt.Errorf("failed to create header for file '%s': %w", relPath, err)
			}
			zipHeader.Name = relPath
			zipHeader.Method = zip.Deflate

			writer, err := zipWriter.CreateHeader(zipHeader)
			if err != nil {
				return fmt.Errorf("failed to create header for file '%s': %w", relPath, err)
			}

			// Copy actual file contents into the zip
			if _, err := io.Copy(writer, file); err != nil {
				return fmt.Errorf("failed to write file '%s' to archive: %w", relPath, err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to pack directory '%s': %w", directoryPath, err)
		}

		fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, fsdlFileName)
		return nil
	},
}
