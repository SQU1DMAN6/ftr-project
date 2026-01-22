package cmd

import (
	"bufio"
	"fmt"
	"ftr/pkg/boxlet"
	"ftr/pkg/builder"
	"ftr/pkg/fsdl"
	"ftr/pkg/registry"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

		// Use fixed temporary directory to mirror `ftr get` behavior
		tmpDir := "/tmp/fsdl"
		// ensure clean workspace
		_ = os.RemoveAll(tmpDir)
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create temporary directory '%s': %w", tmpDir, err)
		}
		defer os.RemoveAll(tmpDir)

		sourceFile, err := os.Open(fsdlFilePath)
		if err != nil {
			return fmt.Errorf("failed to open source file '%s': %w", fsdlFilePath, err)
		}
		defer sourceFile.Close()

		destinationFilePath := filepath.Join(tmpDir, filepath.Base(fsdlFilePath))
		destinationFile, err := os.Create(destinationFilePath)
		if err != nil {
			return fmt.Errorf("failed to create destination file '%s': %w", destinationFilePath, err)
		}
		if _, err := io.Copy(destinationFile, sourceFile); err != nil {
			destinationFile.Close()
			return fmt.Errorf("failed to copy source file to temporary directory: %w", err)
		}
		destinationFile.Close()

		// Extract the contents of the package. Prefer SQAR if the file is a .sqar
		if strings.HasSuffix(strings.ToLower(destinationFilePath), ".sqar") {
			if err := extractSqar(destinationFilePath, tmpDir); err != nil {
				return fmt.Errorf("failed to extract .sqar package: %w", err)
			}
		} else {
			if err := fsdl.Extract(destinationFilePath, tmpDir); err != nil {
				return fmt.Errorf("failed to extract .fsdl package: %w", err)
			}
		}

		// After extraction: determine target arch/os from filename and BUILD/Meta.config
		filename := filepath.Base(destinationFilePath)
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

		// normalize and compare
		normArch := func(a string) string {
			a = strings.TrimSpace(a)
			if a == "amd64" {
				return "x64"
			}
			return a
		}
		normOS := func(o string) string {
			return strings.TrimSpace(strings.ToLower(o))
		}

		localArch := runtime.GOARCH
		if localArch == "amd64" {
			localArch = "x64"
		}
		localOS := runtime.GOOS

		matchesTarget := func(t string, local string, normalize func(string) string) bool {
			if t == "" {
				return true
			}
			for _, tok := range strings.Split(t, ",") {
				tok = normalize(tok)
				if tok == "all" || tok == local {
					return true
				}
			}
			return false
		}

		ta := normArch(targetArch)
		to := normOS(targetOS)
		la := normArch(localArch)
		lo := normOS(localOS)

		okArch := matchesTarget(ta, la, normArch)
		okOS := matchesTarget(to, lo, normOS)
		if !okArch || !okOS {
			fmt.Printf("Warning: package targets %s/%s which does not match your system %s/%s\n", targetArch, targetOS, localArch, localOS)
			fmt.Printf("Proceed? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(ans)) != "y" {
				fmt.Println("Skipping build for", repoName)
				return nil
			}
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

		// Register package in the local registry
		regInfo := registry.PackageInfo{
			Name:        repoName,
			Version:     "",
			Source:      "",
			InstallPath: filepath.Join("/usr/local/share", repoName),
			BinaryPath:  filepath.Join("/usr/local/bin", repoName),
		}
		_ = registry.Register(regInfo)

		// Successful build and installation
		fmt.Println("Build and installation completed successfully.")
		return nil
	},
}
