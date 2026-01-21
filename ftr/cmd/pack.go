package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"ftr/pkg/boxlet"
	"ftr/pkg/sqar"

	"github.com/spf13/cobra"
)

func init() {
	packCmd.Flags().BoolP("use-fsdl", "U", false, "Use FSDL packaging mode instead of SQAR")
	packCmd.Flags().BoolP("sqar-compress", "C", false, "Enable best-level compression when packing with SQAR")
}

var packCmd = &cobra.Command{
	Use:   "pack [directory] [reponame]",
	Short: "Pack a directory into a packaged file",
	Long:  `Pack a project directory into an .sqar (preferred) or .fsdl archive. Example: ftr pack myproject/ myproject`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		directoryPath := args[0]
		repoName := args[1]

		info, err := os.Stat(directoryPath)
		if err != nil {
			return fmt.Errorf("failed to access project directory '%s': %w", directoryPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("'%s' is not a valid directory", directoryPath)
		}

		useFsdl, _ := cmd.Flags().GetBool("use-fsdl")
		sqarCompress, _ := cmd.Flags().GetBool("sqar-compress")

		if useFsdl {
			fsdlFileName := fmt.Sprintf("%s.fsdl", repoName)
			if err := createFsdl(directoryPath, fsdlFileName); err != nil {
				return err
			}
			fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, fsdlFileName)
			return nil
		}

		meta, _ := boxlet.ReadMeta(directoryPath)
		version := "0.0.0"
		arch := "all"
		osName := "all"
		if meta != nil {
			if v, ok := meta["VERSION"]; ok && v != "" {
				version = v
			}
			if a, ok := meta["TARGET_ARCHITECTURE"]; ok && a != "" {
				arch = a
			}
			if o, ok := meta["TARGET_OS"]; ok && o != "" {
				osName = o
			}
		}

		archList := []string{arch}
		osList := []string{osName}
		if strings.Contains(arch, ",") {
			archList = nil
			for _, s := range strings.Split(arch, ",") {
				archList = append(archList, strings.TrimSpace(s))
			}
		}
		if strings.Contains(osName, ",") {
			osList = nil
			for _, s := range strings.Split(osName, ",") {
				osList = append(osList, strings.TrimSpace(s))
			}
		}

		detectedArch := runtime.GOARCH
		if detectedArch == "amd64" {
			detectedArch = "x64"
		}

		sqarTool := sqar.FindSqarTool()

		for _, a := range archList {
			for _, o := range osList {
				useArch := a
				useOS := o

				sqarFileName := fmt.Sprintf("%s-%s-%s-%s.sqar", repoName, version, useArch, useOS)

				tmpSrc, cleanup, err := preparePackSrc(directoryPath)
				if err != nil {
					return fmt.Errorf("failed to prepare source for packing: %w", err)
				}

				if sqarTool == "" {
					fsdlFileName := fmt.Sprintf("%s-%s-%s-%s.fsdl", repoName, version, useArch, useOS)
					if err := createFsdl(tmpSrc, fsdlFileName); err != nil {
						cleanup()
						return err
					}
					fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, fsdlFileName)
					cleanup()
					continue
				}

				if err := createSqar(tmpSrc, sqarFileName, sqarCompress); err != nil {
					cleanup()
					return fmt.Errorf("failed to create SQAR '%s': %w", sqarFileName, err)
				}
				fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, sqarFileName)
				cleanup()
			}
		}
		return nil
	},
}

// preparePackSrc copies srcDir into a temporary directory excluding
// .sqar and .fsdl files, returning the temp dir path and a cleanup func.
func preparePackSrc(srcDir string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "ftr-src-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip generated archives
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".sqar" || ext == ".fsdl" {
				return nil
			}
		}
		dest := filepath.Join(tmpDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		// copy file
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}
	return tmpDir, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func createFsdl(srcDir, destPath string) error {
	// Create temp file outside srcDir to avoid including the archive inside itself
	tmpFile, err := os.CreateTemp("", "ftr-*.fsdl")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", destPath, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmpFile)
	defer zw.Close()

	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == srcDir {
			return nil
		}
		if filepath.Ext(path) == ".fsdl" {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			_, err = zw.Create(rel + "/")
			return err
		}
		fsrc, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsrc.Close()
		fi, _ := fsrc.Stat()
		hdr, _ := zip.FileInfoHeader(fi)
		hdr.Name = rel
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, fsrc)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", destPath, err)
	}

	// Move temp file to final destination (overwrite if exists)
	if err := os.Rename(tmpPath, destPath); err != nil {
		// try copy fallback
		in, err2 := os.Open(tmpPath)
		if err2 != nil {
			return fmt.Errorf("failed to move temp fsdl to destination: %w", err)
		}
		defer in.Close()
		out, err2 := os.Create(destPath)
		if err2 != nil {
			return fmt.Errorf("failed to create destination fsdl: %w", err2)
		}
		defer out.Close()
		if _, err2 := io.Copy(out, in); err2 != nil {
			return fmt.Errorf("failed to copy fsdl to destination: %w", err2)
		}
		_ = os.Remove(tmpPath)
	}
	return nil
}

func createSqar(srcDir, destPath string, sqarCompress bool) error {
	sqarTool := sqar.FindSqarTool()
	if sqarTool == "" {
		return fmt.Errorf("sqar utility not found")
	}

	// Use a temp output path outside srcDir to avoid the archive being included
	tmpFile, err := os.CreateTemp("", "ftr-*.sqar")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", destPath, err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	var cmd *exec.Cmd
	if sqarCompress {
		cmd = exec.Command(sqarTool, "pack", "-C", "-L", "best", "-I", srcDir, "-O", tmpPath)
	} else {
		cmd = exec.Command(sqarTool, "pack", "-I", srcDir, "-O", tmpPath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Move temp sqar to final destination (overwrite if exists)
	if err := os.Rename(tmpPath, destPath); err != nil {
		// copy fallback
		in, err2 := os.Open(tmpPath)
		if err2 != nil {
			return fmt.Errorf("failed to move temp sqar to destination: %w", err)
		}
		defer in.Close()
		out, err2 := os.Create(destPath)
		if err2 != nil {
			return fmt.Errorf("failed to create destination sqar: %w", err2)
		}
		defer out.Close()
		if _, err2 := io.Copy(out, in); err2 != nil {
			return fmt.Errorf("failed to copy sqar to destination: %w", err2)
		}
		_ = os.Remove(tmpPath)
	}
	return nil
}
