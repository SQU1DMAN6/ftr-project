package builder

import (
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	return boxlet.ReadMeta(b.WorkDir)
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
		filepath.Join(homeDir, ".local", "share"),
		filepath.Join(homeDir, "bin"),
	}

	for _, d := range scanDirs {
		_ = filepath.WalkDir(d, func(p string, de os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if de.IsDir() {
				return nil
			}
			info, err := de.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(after) {
				newFiles = append(newFiles, p)
			}
			return nil
		})
	}
	return newFiles
}

// DetectAndBuild attempts to detect how to build or install the package and
// returns a path to a produced binary (or installer) when available.
func (b *Builder) DetectAndBuild() (string, error) {
	// 1) Prefer pre-built linux binaries placed under BUILD/linux-{arch}
	// Detect current system architecture
	currentArch := runtime.GOARCH
	if currentArch == "amd64" {
		currentArch = "x64"
	} else if currentArch == "arm64" {
		currentArch = "arm64"
	} else if currentArch == "arm" {
		currentArch = "arm"
	}

	archSpecificDirs := []string{
		filepath.Join(b.WorkDir, "BUILD", fmt.Sprintf("linux-%s", currentArch)),
		filepath.Join(b.WorkDir, "BUILD", "linux"),
	}

	for _, linuxDir := range archSpecificDirs {
		if info, err := os.Stat(linuxDir); err == nil && info.IsDir() {
			if entries, err := os.ReadDir(linuxDir); err == nil {
				var found string
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					if e.Name() == b.RepoName {
						found = e.Name()
						break
					}
				}
				if found == "" {
					for _, e := range entries {
						if !e.IsDir() {
							found = e.Name()
							break
						}
					}
				}
				if found != "" {
					return filepath.Join(linuxDir, found), nil
				}
			}
		}
	}

	// 2) Detect Windows MSI placed under BUILD/windows (returned for upload/install handling)
	windowsDir := filepath.Join(b.WorkDir, "BUILD", "windows")
	if info, err := os.Stat(windowsDir); err == nil && info.IsDir() {
		if entries, err := os.ReadDir(windowsDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.HasSuffix(strings.ToLower(e.Name()), ".msi") {
					return filepath.Join(windowsDir, e.Name()), nil
				}
			}
		}
	}

	// 3) Load metadata (BUILD/fsdlbuild.ftr or BUILD/Meta.config) and honour
	// BUILD_COMMAND / INSTALL_COMMAND and explicit output paths.
	meta, _ := b.loadBuildMeta()
	if meta != nil {
		if buildCmd, ok := meta["BUILD_COMMAND"]; ok && strings.TrimSpace(buildCmd) != "" {
			fmt.Println("Detected build meta custom build command.")
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

		if installCmd, ok := meta["INSTALL_COMMAND"]; ok && strings.TrimSpace(installCmd) != "" {
			fmt.Println("Detected build meta install command.")
			if err := b.run(installCmd); err != nil {
				return "", fmt.Errorf("custom install command failed: %w", err)
			}
			// install command may have installed files to system locations
			return "", nil
		}
	}

	// 4) Prefer install.sh if present
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		fmt.Println("install.sh found. Running and scanning for installed files...")
		before := time.Now().Add(-1 * time.Second)
		if err := b.run("sed -i 's/\\r$//' install.sh && chmod +x install.sh && ./install.sh"); err != nil {
			return "", fmt.Errorf("install.sh failed: %w", err)
		}
		b.InstallPaths = scanForNewFiles(before)
		for _, p := range b.InstallPaths {
			if strings.HasPrefix(p, "/usr/local/bin/") && !strings.HasSuffix(p, "/") {
				return p, nil
			}
		}
		return "", nil
	}

	// 5) Check for a prebuilt binary named after the repo in the workdir
	if _, err := os.Stat(filepath.Join(b.WorkDir, b.RepoName)); err == nil {
		if f, err := elf.Open(filepath.Join(b.WorkDir, b.RepoName)); err == nil {
			if f.Type == elf.ET_EXEC || f.Type == elf.ET_DYN {
				return filepath.Join(b.WorkDir, b.RepoName), nil
			}
		}
	}

	// 6) Makefile
	if _, err := os.Stat(filepath.Join(b.WorkDir, "Makefile")); err == nil {
		fmt.Println("Makefile found. Running make...")
		if err := b.run("make"); err != nil {
			return "", fmt.Errorf("make failed: %w", err)
		}
		if _, err := os.Stat(filepath.Join(b.WorkDir, b.RepoName)); err == nil {
			return filepath.Join(b.WorkDir, b.RepoName), nil
		}
		return "", nil
	}

	// 7) Python app
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.py")); err == nil {
		fmt.Println("Detected Python app. Building with PyInstaller...")
		if err := b.run("pip install pyinstaller"); err != nil {
			return "", fmt.Errorf("failed to install pyinstaller: %w", err)
		}
		hiddenImports := []string{"pyttsx3", "pkg_resources.py2_warn", "engine", "comtypes", "dnspython", "sympy", "numpy"}
		importFlags := ""
		for _, imp := range hiddenImports {
			importFlags += fmt.Sprintf(" --hidden-import=%s", imp)
		}
		buildCmd := fmt.Sprintf("pyinstaller --noconsole --onefile main.py --name %s %s", b.RepoName, importFlags)
		if err := b.run(buildCmd); err != nil {
			return "", fmt.Errorf("pyinstaller failed: %w", err)
		}
		binaryPath := filepath.Join(b.WorkDir, "dist", b.RepoName)
		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath, nil
		}
		return "", fmt.Errorf("binary not found after build")
	}

	// 8) Go app
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.go")); err == nil {
		fmt.Println("Detected Go app. Building...")
		if err := b.run(fmt.Sprintf("go build -o %s .", b.RepoName)); err != nil {
			return "", fmt.Errorf("go build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// 9) C++ app
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cpp")); err == nil {
		fmt.Println("Detected C++ app. Building with g++...")
		if err := b.run(fmt.Sprintf("g++ main.cpp -o %s", b.RepoName)); err != nil {
			return "", fmt.Errorf("g++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// 10) SQU1D
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.sqd")); err == nil {
		fmt.Println("Detected SQU1D app. Building with squ1d++...")
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

	ext := strings.ToLower(filepath.Ext(binaryPath))
	if ext == ".msi" || ext == ".exe" {
		if err := os.MkdirAll(shareDir, 0755); err != nil {
			home, _ := os.UserHomeDir()
			shareDir = filepath.Join(home, ".local", "share", b.RepoName)
			_ = os.MkdirAll(shareDir, 0755)
		}
		in, err := os.Open(binaryPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to open installer: %w", err)
		}
		defer in.Close()
		outPath := filepath.Join(shareDir, binName)
		out, err := os.Create(outPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to copy installer: %w", err)
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return "", "", fmt.Errorf("failed to copy installer: %w", err)
		}
		fmt.Printf("Installed as '%s' (copied to %s)\n", binName, outPath)
		return "", shareDir, nil
	}

	if err := b.run(fmt.Sprintf("chmod +x %s", binaryPath)); err != nil {
		return "", "", fmt.Errorf("chmod failed: %w", err)
	}
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
