package builder

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"inker4win/pkg/boxlet"
)

var ErrPrivilegedRequired = errors.New("privileged actions required")
var ErrDelegatedToTerminal = errors.New("delegated to terminal")

// Builder handles Windows-specific package building and installation
type Builder struct {
	RepoName     string
	WorkDir      string
	InstallPaths []string // Paths created during installation
}

// New creates a new Builder instance for Windows
func New(repoName, workDir string) *Builder {
	if strings.TrimSpace(workDir) == "" {
		workDir = filepath.Join(os.TempDir(), "inker-pkgs")
	}
	// Ensure workDir exists
	_ = os.MkdirAll(workDir, 0755)
	return &Builder{
		RepoName: repoName,
		WorkDir:  workDir,
	}
}

// run executes a command and returns error if any
func (b *Builder) run(cmd string) error {
	command := exec.Command("powershell", "-NoProfile", "-Command", cmd)
	command.Dir = b.WorkDir
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// DetectAndBuild handles Windows-specific package detection and building
func (b *Builder) DetectAndBuild() (string, error) {
	var foundBinary string

	// Early: detect prebuilt .exe in workdir and skip building if present
	entries, _ := os.ReadDir(b.WorkDir)
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			name := e.Name()
			if foundBinary == "" || strings.Contains(strings.ToLower(name), strings.ToLower(b.RepoName)) {
				foundBinary = filepath.Join(b.WorkDir, name)
			}
		}
	}
	if foundBinary != "" {
		log.Printf("Prebuilt executable detected: %s\n", foundBinary)
		return foundBinary, nil
	}

	// Check for install.bat or install.ps1 first
	installBat := filepath.Join(b.WorkDir, "install.bat")
	installPS1 := filepath.Join(b.WorkDir, "install.ps1")

	b.InstallPaths = []string{}

	// Prefer PowerShell script over batch file
	if _, err := os.Stat(installPS1); err == nil {
		log.Println("install.ps1 found. Running Windows installer...")

		// Run PowerShell script
		if err := b.run(fmt.Sprintf("& '%s'", installPS1)); err != nil {
			return "", fmt.Errorf("install.ps1 failed: %w", err)
		}

		return foundBinary, nil
	}

	// Fall back to batch file
	if _, err := os.Stat(installBat); err == nil {
		log.Println("install.bat found. Running Windows batch installer...")

		// Run batch script
		if err := b.run(installBat); err != nil {
			return "", fmt.Errorf("install.bat failed: %w", err)
		}

		return foundBinary, nil
	}

	// Check for main.cs (C# / .NET)
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cs")); err == nil {
		log.Println("Detected C# app. Building with csc.exe...")
		if err := b.run(fmt.Sprintf("csc main.cs -out:%s.exe", b.RepoName)); err != nil {
			return "", fmt.Errorf("C# build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, fmt.Sprintf("%s.exe", b.RepoName)), nil
	}

	// Check for main.py (Python)
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.py")); err == nil {
		log.Println("Detected Python app. Building with PyInstaller...")
		if err := b.run("pip install pyinstaller"); err != nil {
			return "", fmt.Errorf("failed to install pyinstaller: %w", err)
		}

		buildCmd := fmt.Sprintf("pyinstaller --onefile --name %s main.py", b.RepoName)
		if err := b.run(buildCmd); err != nil {
			return "", fmt.Errorf("pyinstaller failed: %w", err)
		}

		binaryPath := filepath.Join(b.WorkDir, "dist", fmt.Sprintf("%s.exe", b.RepoName))
		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath, nil
		}
		return "", fmt.Errorf("binary not found after build")
	}

	// Check for main.go (Go)
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.go")); err == nil {
		log.Println("Detected Go app. Building...")
		if err := b.run(fmt.Sprintf("go build -o %s.exe .", b.RepoName)); err != nil {
			return "", fmt.Errorf("go build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, fmt.Sprintf("%s.exe", b.RepoName)), nil
	}

	// Check for main.cpp (C++)
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cpp")); err == nil {
		log.Println("Detected C++ app. Building with g++...")
		if err := b.run(fmt.Sprintf("g++ main.cpp -o %s.exe", b.RepoName)); err != nil {
			return "", fmt.Errorf("g++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, fmt.Sprintf("%s.exe", b.RepoName)), nil
	}

	// No installation method found
	return "", fmt.Errorf("no supported Windows installer found (install.ps1, install.bat, main.cs, main.py, main.go, or main.cpp)")
}

// InstallBinary handles installation of Windows binaries
// On Windows, prefer user AppData bin directory to avoid UAC prompts
// Falls back to Program Files if metadata indicates system-wide installation is required
func (b *Builder) InstallBinary(binaryPath string) error {
	// Read BUILD/Meta.config to determine install behavior
	meta, _ := boxlet.ReadMeta(b.WorkDir)

	// Prefer user bin directory first to avoid admin privileges
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}
	userBin := filepath.Join(userProfile, "bin")

	// If INSTALL_AS_CLI is present, install the executable into the user's bin and add to PATH
	if meta != nil {
		if installCli, ok := meta["INSTALL_AS_CLI"]; ok && strings.TrimSpace(installCli) != "" {
			cliName := strings.TrimSpace(meta["CLI_COMMAND_NAME"])
			if cliName == "" {
				cliName = strings.TrimSuffix(filepath.Base(binaryPath), filepath.Ext(binaryPath))
			}

			if err := os.MkdirAll(userBin, 0755); err != nil {
				return fmt.Errorf("failed to create user bin directory: %w", err)
			}

			dest := filepath.Join(userBin, cliName+".exe")
			content, err := os.ReadFile(binaryPath)
			if err != nil {
				return fmt.Errorf("failed to read binary: %w", err)
			}
			if err := os.WriteFile(dest, content, 0755); err != nil {
				return fmt.Errorf("failed to copy binary to user bin: %w", err)
			}

			// Add user bin to user's PATH if not already present
			// Use PowerShell to update the User environment PATH
			addPathCmd := fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';%s', 'User')", userBin)
			_ = exec.Command("powershell", "-NoProfile", "-Command", addPathCmd).Run()

			// Create a data directory for the package under user AppData
			dataDir := filepath.Join(userProfile, "AppData", "Local", "FtR", "packages", b.RepoName)
			_ = os.MkdirAll(dataDir, 0755)

			log.Printf("Installed CLI '%s' to %s\n", cliName, dest)
			return nil
		}
	}

	// Default installation: copy executable into user bin directory first
	if err := os.MkdirAll(userBin, 0755); err == nil {
		binName := filepath.Base(binaryPath)
		destBin := filepath.Join(userBin, binName)
		content, err := os.ReadFile(binaryPath)
		if err == nil && os.WriteFile(destBin, content, 0755) == nil {
			log.Printf("Installed %s to %s\n", binName, destBin)
			return nil
		}
	}

	// Fall back to Program Files if user bin fails
	programFiles := "C:\\Program Files\\FtR\\packages"
	if err := os.MkdirAll(programFiles, 0755); err != nil {
		return fmt.Errorf("failed to create installation directory: %w", err)
	}

	binName := filepath.Base(binaryPath)
	destBin := filepath.Join(programFiles, binName)

	// Copy binary to installation directory
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	if err := os.WriteFile(destBin, content, 0755); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Create data directory for the package
	dataDir := filepath.Join(programFiles, b.RepoName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	log.Printf("Installed %s to %s\n", binName, destBin)
	return nil
}

// SetPassword sets the password for privileged execution (no-op on Windows)
func (b *Builder) SetPassword(password string) {
}

// ExecutePrivileged executes privileged commands (no-op on Windows)
func (b *Builder) ExecutePrivileged() error {
	return nil
}
