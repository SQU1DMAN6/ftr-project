package fsdl

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Package represents an FSDL package
type Package struct {
	Name    string
	Files   []File
	TempDir string
}

// File represents a file in the package
type File struct {
	Name string
	Path string
	Size int64
}

// Create creates a new FSDL package from a directory
func Create(name, sourcePath string) (*Package, error) {
	pkg := &Package{
		Name:  name,
		Files: []File{},
	}

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			pkg.Files = append(pkg.Files, File{
				Name: filepath.Base(path),
				Path: relPath,
				Size: info.Size(),
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return pkg, nil
}

// Pack creates an FSDL file from the package
func (p *Package) Pack(sourcePath, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, file := range p.Files {
		err := func() error {
			srcFile, err := os.Open(filepath.Join(sourcePath, file.Path))
			if err != nil {
				return fmt.Errorf("failed to open source file %s: %w", file.Path, err)
			}
			defer srcFile.Close()

			destFile, err := zipWriter.Create(file.Path)
			if err != nil {
				return fmt.Errorf("failed to create zip entry for %s: %w", file.Path, err)
			}

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return fmt.Errorf("failed to copy file contents for %s: %w", file.Path, err)
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

// Extract extracts an FSDL package to a directory
func Extract(fsdlPath, destPath string) error {
	reader, err := zip.OpenReader(fsdlPath)
	if err != nil {
		return fmt.Errorf("failed to open FSDL file: %w", err)
	}
	defer reader.Close()

	err = os.MkdirAll(destPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, file := range reader.File {
		err := func() error {
			if file.FileInfo().IsDir() {
				err := os.MkdirAll(filepath.Join(destPath, file.Name), 0755)
				if err != nil {
					return fmt.Errorf("failed to create directory %s: %w", file.Name, err)
				}
				return nil
			}

			outPath := filepath.Join(destPath, file.Name)
			err := os.MkdirAll(filepath.Dir(outPath), 0755)
			if err != nil {
				return fmt.Errorf("failed to create parent directories for %s: %w", file.Name, err)
			}

			outFile, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", file.Name, err)
			}
			defer outFile.Close()

			rc, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open .fsdl file. If this file is encrypted, ensure you have the key by logging in to FtR")
			}
			defer rc.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}
