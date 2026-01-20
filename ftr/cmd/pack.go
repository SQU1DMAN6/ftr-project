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

				if sqarTool == "" {
					fsdlFileName := fmt.Sprintf("%s-%s-%s-%s.fsdl", repoName, version, useArch, useOS)
					if err := createFsdl(directoryPath, fsdlFileName); err != nil {
						return err
					}
					fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, fsdlFileName)
					continue
				}

				if err := createSqar(directoryPath, sqarFileName, sqarCompress); err != nil {
					return fmt.Errorf("failed to create SQAR '%s': %w", sqarFileName, err)
				}
				fmt.Printf("Successfully packed '%s' into '%s'\n", directoryPath, sqarFileName)
			}
		}
		return nil
	},
}

func createFsdl(srcDir, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", destPath, err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
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
	return nil
}

func createSqar(srcDir, destPath string, sqarCompress bool) error {
	sqarTool := sqar.FindSqarTool()
	if sqarTool == "" {
		return fmt.Errorf("sqar utility not found")
	}

	var cmd *exec.Cmd
	if sqarCompress {
		cmd = exec.Command(sqarTool, "pack", "-C", "-L", "best", "-I", srcDir, "-O", destPath)
	} else {
		cmd = exec.Command(sqarTool, "pack", "-I", srcDir, "-O", destPath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
