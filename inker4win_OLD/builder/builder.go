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

	"golang.org/x/sys/windows/registry"
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

// createStartMenuEntry creates a Windows Start Menu shortcut for the installed application
func (b *Builder) createStartMenuEntry(binaryPath, displayName string) error {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}

	startMenuPath := filepath.Join(userProfile, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "FtR")
	if err := os.MkdirAll(startMenuPath, 0755); err != nil {
		return fmt.Errorf("failed to create start menu directory: %w", err)
	}

	shortcutPath := filepath.Join(startMenuPath, displayName+".lnk")

	// Use PowerShell to create a shortcut (.lnk file)
	psCmd := fmt.Sprintf(`
		$WshShell = New-Object -ComObject WScript.Shell
		$Shortcut = $WshShell.CreateShortcut('%s')
		$Shortcut.TargetPath = '%s'
		$Shortcut.WorkingDirectory = (Split-Path -Parent '%s')
		$Shortcut.Save()
	`, shortcutPath, binaryPath, binaryPath)

	// Run PowerShell command to create shortcut
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create start menu shortcut: %w", err)
	}

	log.Printf("Created start menu entry at %s\n", shortcutPath)
	return nil
}

// writeVersionToRegistry writes the application version to the Windows registry
func (b *Builder) writeVersionToRegistry(appName, version string) error {
	// Write to HKEY_CURRENT_USER for user-specific installation
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\FtR\Packages`, registry.ALL_ACCESS)
	if err != nil {
		// If key doesn't exist, create it
		key, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\FtR\Packages`, registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("failed to create registry key: %w", err)
		}
	}
	defer key.Close()

	// Create subkey for the application
	appKey, _, err := registry.CreateKey(key, appName, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create app registry key: %w", err)
	}
	defer appKey.Close()

	// Write version to registry
	if err := appKey.SetStringValue("Version", version); err != nil {
		return fmt.Errorf("failed to write version to registry: %w", err)
	}

	log.Printf("Wrote version '%s' to registry for '%s'\n", version, appName)
	return nil
}

// DetectAndBuild handles Windows-specific package detection and building
func (b *Builder) DetectAndBuild() (string, error) {
	var foundBinary string

	// Early: detect prebuilt .exe in workdir and skip building if present
	// First, look for an exact match: <reponame>.exe
	expectedBinaryName := fmt.Sprintf("%s.exe", b.RepoName)
	expectedBinaryPath := filepath.Join(b.WorkDir, expectedBinaryName)
	if _, err := os.Stat(expectedBinaryPath); err == nil {
		log.Printf("Prebuilt executable detected: %s\n", expectedBinaryPath)
		return expectedBinaryPath, nil
	}

	// Fall back to searching for any .exe that contains the repo name
	entries, _ := os.ReadDir(b.WorkDir)
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			name := e.Name()
			if strings.Contains(strings.ToLower(name), strings.ToLower(b.RepoName)) {
				foundBinary = filepath.Join(b.WorkDir, name)
				log.Printf("Prebuilt executable detected: %s\n", foundBinary)
				return foundBinary, nil
			}
		}
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

// safeInstall copies a file from src to dst, renaming any existing dst to .old first.
func safeInstall(src, dst string) error {
	renamed := false
	oldPath := dst + ".old"

	// Check if destination exists
	if _, err := os.Stat(dst); err == nil {
		// Try to remove any previous .old file
		_ = os.Remove(oldPath)

		// Rename current file to .old
		if err := os.Rename(dst, oldPath); err != nil {
			return err
		}
		renamed = true
	}

	content, err := os.ReadFile(src)
	if err != nil {
		if renamed {
			_ = os.Rename(oldPath, dst)
		}
		return err
	}

	if err := os.WriteFile(dst, content, 0755); err != nil {
		if renamed {
			_ = os.Rename(oldPath, dst)
		}
		return err
	}

	// Attempt to remove the .old file when done
	if renamed {
		_ = os.Remove(oldPath)
	}

	return nil
}

// InstallBinary handles installation of Windows binaries
// On Windows, prefer user AppData bin directory to avoid UAC prompts
// Falls back to Program Files if metadata indicates system-wide installation is required
// Supports START_MENU_ENTRY for application shortcuts and VERSION registry entries
func (b *Builder) InstallBinary(binaryPath string) error {
	// Read BUILD/Meta.config to determine install behavior
	meta, _ := boxlet.ReadMeta(b.WorkDir)

	// Prefer user bin directory first to avoid admin privileges
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}
	userBin := filepath.Join(userProfile, "bin")

	// Extract display name and version from metadata
	displayName := strings.TrimSpace(meta["DISPLAY_NAME"])
	if displayName == "" {
		displayName = b.RepoName
	}
	version := strings.TrimSpace(meta["VERSION"])
	if version == "" {
		version = "1.0.0"
	}

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
			if err := safeInstall(binaryPath, dest); err != nil {
				return fmt.Errorf("failed to copy binary to user bin: %w", err)
			}

			// Add user bin to user's PATH if not already present
			// Use PowerShell to update the User environment PATH
			addPathCmd := fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';%s', 'User')", userBin)
			_ = exec.Command("powershell", "-NoProfile", "-Command", addPathCmd).Run()

			// Create a data directory for the package under user AppData
			dataDir := filepath.Join(userProfile, "AppData", "Local", "FtR", "packages", b.RepoName)
			_ = os.MkdirAll(dataDir, 0755)

			// Write version to registry
			if version != "" {
				_ = b.writeVersionToRegistry(b.RepoName, version)
			}

			log.Printf("Installed CLI '%s' to %s\n", cliName, dest)
			return nil
		}
	}

	// Default installation: copy executable into user bin directory first
	if err := os.MkdirAll(userBin, 0755); err == nil {
		binName := filepath.Base(binaryPath)
		destBin := filepath.Join(userBin, binName)
		if err := safeInstall(binaryPath, destBin); err == nil {
			// Create start menu entry if flagged
			if meta != nil {
				if startMenuEntry, ok := meta["START_MENU_ENTRY"]; ok && strings.TrimSpace(startMenuEntry) != "" {
					_ = b.createStartMenuEntry(destBin, displayName)
				}
			}

			// Write version to registry
			if version != "" {
				_ = b.writeVersionToRegistry(b.RepoName, version)
			}

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
	if err := safeInstall(binaryPath, destBin); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Create data directory for the package
	dataDir := filepath.Join(programFiles, b.RepoName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create start menu entry if flagged
	if meta != nil {
		if startMenuEntry, ok := meta["START_MENU_ENTRY"]; ok && strings.TrimSpace(startMenuEntry) != "" {
			_ = b.createStartMenuEntry(destBin, displayName)
		}
	}

	// Write version to registry
	if version != "" {
		_ = b.writeVersionToRegistry(b.RepoName, version)
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
