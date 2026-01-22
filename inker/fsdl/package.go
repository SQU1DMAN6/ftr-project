package fsdl

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Package struct {
	Name    string
	Files   []File
	TempDir string
}

type File struct {
	Name string
	Path string
	Size int64
}

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

func Extract(fsdlPath, destPath string) error {
	reader, err := zip.OpenReader(fsdlPath)
	if err != nil {
		return fmt.Errorf("failed to open FSDL file. Consider logging in if it is encrypted, or contacting the developer")
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
