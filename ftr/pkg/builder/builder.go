package builder

import (
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ftr/pkg/boxlet"
)

// Builder handles the detection and building of different project types
type Builder struct {
	RepoName     string
	WorkDir      string
	InstallPaths []string // Paths created during install.sh execution
}

// New creates a new Builder instance
func New(repoName, workDir string) *Builder {
	if strings.TrimSpace(workDir) == "" {
		workDir = "/tmp/fsdl"
	}
	// Ensure workDir exists so install scripts that cd to /tmp/fsdl succeed
	_ = os.MkdirAll(workDir, 0755)
	return &Builder{
		RepoName: repoName,
		WorkDir:  workDir,
	}
}

// run executes a command and returns error if any
func (b *Builder) run(cmd string) error {
	command := exec.Command("sh", "-c", cmd)
	command.Dir = b.WorkDir
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func (b *Builder) loadBuildMeta() (boxlet.MetaKeyValue, error) {
	meta, err := boxlet.ReadMeta(b.WorkDir)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func metaOutputPath(meta boxlet.MetaKeyValue) string {
	if meta == nil {
		return ""
	}
	for _, key := range []string{"BUILD_OUTPUT", "OUTPUT", "ENTRY_POINT"} {
		if value, ok := meta[key]; ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// scanForNewFiles finds all files modified after the given time in common install directories
func scanForNewFiles(after time.Time) []string {
	var newFiles []string
	homeDir, _ := os.UserHomeDir()

	// Directories to scan for file modifications (more targeted)
	scanDirs := []string{
		"/usr/local/bin",
		"/usr/local/share",
		"/opt",
		"/usr/bin",
		"/etc",
	}

	// For home directory, only scan specific subdirectories
	homeSubdirs := []string{
		filepath.Join(homeDir, ".config"),
		filepath.Join(homeDir, ".local"),
		filepath.Join(homeDir, ".opt"),
	}
	scanDirs = append(scanDirs, homeSubdirs...)

	// Map to track parent directories we've already added (avoid duplicates)
	seenPaths := make(map[string]bool)

	for _, dir := range scanDirs {
		if _, err := os.Stat(dir); err != nil {
			continue // Skip directories that don't exist
		}

		// Walk the directory looking for files modified after 'after'
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Skip certain patterns that are too noisy
			if strings.Contains(path, "/.cache/") ||
				strings.Contains(path, "go/telemetry") ||
				strings.Contains(path, ".npm") {
				return nil
			}

			// Include files and directories modified after the install started
			// But skip top-level scan directories themselves (they change when files are added)
			if info.ModTime().After(after) && path != dir {
				// Only add if we haven't seen this path before (avoid duplicates)
				if !seenPaths[path] {
					newFiles = append(newFiles, path)
					seenPaths[path] = true
				}
			}
			return nil
		})
	}

	return newFiles
}

// DetectAndBuild tries to detect the project type and build it accordingly
func (b *Builder) DetectAndBuild() (string, error) {
	meta, _ := b.loadBuildMeta()
	if meta != nil {
		buildCmd, hasBuildCmd := meta["BUILD_COMMAND"]
		installCmd, hasInstallCmd := meta["INSTALL_COMMAND"]
		if hasBuildCmd && strings.TrimSpace(buildCmd) != "" {
			fmt.Println("Detected BUILD/Meta.config custom build command.")
			if err := b.run(buildCmd); err != nil {
				return "", fmt.Errorf("custom build command failed: %w", err)
			}
		}

		outputPath := metaOutputPath(meta)
		if outputPath != "" {
			resolved := filepath.Join(b.WorkDir, outputPath)
			if _, err := os.Stat(resolved); err == nil {
				return resolved, nil
			}
		}

		if hasInstallCmd && strings.TrimSpace(installCmd) != "" {
			fmt.Println("Detected BUILD/Meta.config install command.")
			if err := b.run(installCmd); err != nil {
				return "", fmt.Errorf("custom install command failed: %w", err)
			}
			if outputPath != "" {
				resolved := filepath.Join(b.WorkDir, outputPath)
				if _, err := os.Stat(resolved); err == nil {
					return resolved, nil
				}
			}
			return "", nil
		}
	}

	// Check for install.sh first
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		fmt.Println("\ninstall.sh found. Running and skipping default installer protocol...")

		// Record the time just before running install.sh to track what gets created
		beforeTime := time.Now().Add(-1 * time.Second)

		// Fix line endings (convert CRLF to LF) in case the script was packaged on Windows
		if err := b.run("sed -i 's/\\r$//' install.sh && chmod +x install.sh && ./install.sh"); err != nil {
			return "", fmt.Errorf("install.sh failed: %w", err)
		}

		// Scan for all new/modified files created by install.sh
		b.InstallPaths = scanForNewFiles(beforeTime)

		// Also extract the binary path if it's a standard location
		var foundBinary string
		for _, path := range b.InstallPaths {
			if strings.HasPrefix(path, "/usr/local/bin/") && !strings.HasSuffix(path, "/") {
				// Prefer the first file in /usr/local/bin as the main binary
				if foundBinary == "" {
					foundBinary = path
				}
			}
		}

		// Return the binary path if found
		return foundBinary, nil
	}

	// Check for pre-built binary (ELF, follows repository name)
	_, err := os.Stat(filepath.Join(b.WorkDir, b.RepoName))
	if err == nil {
		if f, err := elf.Open(filepath.Join(b.WorkDir, b.RepoName)); err == nil {
			fmt.Println("Binary found. Checking binary...")
			binaryPath := filepath.Join(b.WorkDir, b.RepoName)
			if f.Type == elf.ET_EXEC || f.Type == elf.ET_DYN {
				fmt.Println("Proceeding to install binary file...")
				return binaryPath, nil
			}
		}
	}

	// Check for Makefile
	if _, err := os.Stat(filepath.Join(b.WorkDir, "Makefile")); err == nil {
		fmt.Println("\nMakefile found. Running make...")
		if err := b.run("make"); err != nil {
			return "", fmt.Errorf("make failed: %w", err)
		}
		return "", nil
	}

	// Check for main.py
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.py")); err == nil {
		fmt.Println("\nDetected Python app. Building with PyInstaller...")
		if err := b.run("pip install pyinstaller"); err != nil {
			return "", fmt.Errorf("failed to install pyinstaller: %w", err)
		}

		// Add common hidden imports
		hiddenImports := []string{
			"pyttsx3", "pkg_resources.py2_warn", "engine",
			"comtypes", "dnspython", "sympy", "numpy",
		}
		importFlags := ""
		for _, imp := range hiddenImports {
			// TODO: Check if module exists before adding
			importFlags += fmt.Sprintf(" --hidden-import=%s", imp)
		}

		buildCmd := fmt.Sprintf(
			"sudo pyinstaller --noconsole --onefile main.py --name %s %s",
			b.RepoName, importFlags,
		)
		if err := b.run(buildCmd); err != nil {
			return "", fmt.Errorf("pyinstaller failed: %w", err)
		}

		binaryPath := filepath.Join(b.WorkDir, "dist", b.RepoName)
		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath, nil
		}
		return "", fmt.Errorf("binary not found after build")
	}

	// Check for main.go
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.go")); err == nil {
		fmt.Println("\nDetected Go app. Building...")
		if err := b.run(fmt.Sprintf("go build -o %s .", b.RepoName)); err != nil {
			return "", fmt.Errorf("go build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.cpp
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cpp")); err == nil {
		fmt.Println("\nDetected C++ app. Building with g++...")
		if err := b.run(fmt.Sprintf("g++ main.cpp -o %s", b.RepoName)); err != nil {
			return "", fmt.Errorf("g++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.sqd
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.sqd")); err == nil {
		fmt.Println("\nDetected SQU1D app. Building with squ1d++...")
		if err := b.run(fmt.Sprintf("squ1d++ -B main.sqd -o %s", b.RepoName)); err != nil {
			return "", fmt.Errorf("squ1d++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	return "", fmt.Errorf("no known entry point found")
}

// InstallBinary installs the built binary to system directories and returns
// the actual paths where the binary and share directory were installed
func (b *Builder) InstallBinary(binaryPath string) (binaryPathOut, sharePathOut string, err error) {
	binName := filepath.Base(binaryPath)
	destBin := filepath.Join("/usr/local/bin", binName)
	shareDir := filepath.Join("/usr/local/share", b.RepoName)

	// Make binary executable
	if err := b.run(fmt.Sprintf("chmod +x %s", binaryPath)); err != nil {
		return "", "", fmt.Errorf("chmod failed: %w", err)
	}

	// Copy to /usr/local/bin
	if err := b.run(fmt.Sprintf("sudo cp -r %s %s", binaryPath, destBin)); err != nil {
		return "", "", fmt.Errorf("failed to copy to /usr/local/bin: %w", err)
	}

	if err := b.run(fmt.Sprintf("sudo mkdir -p %s", shareDir)); err != nil {
		return "", "", fmt.Errorf("failed to create share directory: %w", err)
	}
	if err := b.run(fmt.Sprintf("sudo cp -r %s %s", binaryPath, shareDir)); err != nil {
		return "", "", fmt.Errorf("failed to copy to share directory: %w", err)
	}

	fmt.Printf("Installed as '%s'\n", binName)
	return destBin, shareDir, nil
}
