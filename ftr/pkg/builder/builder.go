package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Builder handles the detection and building of different project types
type Builder struct {
	RepoName string
	WorkDir  string
}

// New creates a new Builder instance
func New(repoName, workDir string) *Builder {
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

// DetectAndBuild tries to detect the project type and build it accordingly
func (b *Builder) DetectAndBuild() (string, error) {
	// Check for install.sh first
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		fmt.Println("install.sh found. Running and skipping default installer protocol...")
		if err := b.run("chmod +x install.sh && ./install.sh"); err != nil {
			return "", fmt.Errorf("install.sh failed: %w", err)
		}
		return "", nil
	}

	// Check for Makefile
	if _, err := os.Stat(filepath.Join(b.WorkDir, "Makefile")); err == nil {
		fmt.Println("Makefile found. Running make...")
		if err := b.run("make"); err != nil {
			return "", fmt.Errorf("make failed: %w", err)
		}
		return "", nil
	}

	// Check for main.py
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.py")); err == nil {
		fmt.Println("Detected Python app. Building with PyInstaller...")
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
		fmt.Println("Detected Go app. Building...")
		if err := b.run(fmt.Sprintf("go build -o %s .", b.RepoName)); err != nil {
			return "", fmt.Errorf("go build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.cpp
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cpp")); err == nil {
		fmt.Println("Detected C++ app. Building with g++...")
		if err := b.run(fmt.Sprintf("g++ main.cpp -o %s", b.RepoName)); err != nil {
			return "", fmt.Errorf("g++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.sqd
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.sqd")); err == nil {
		fmt.Println("Detected SQU1D app. Building with squ1d++...")
		if err := b.run(fmt.Sprintf("squ1d++ -B main.sqd -o %s", b.RepoName)); err != nil {
			return "", fmt.Errorf("squ1d++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	return "", fmt.Errorf("no known entry point found")
}

// InstallBinary installs the built binary to system directories
func (b *Builder) InstallBinary(binaryPath string) error {
	binName := filepath.Base(binaryPath)
	destBin := filepath.Join("/usr/local/bin", binName)
	shareDir := filepath.Join("/usr/local/share", b.RepoName)

	// Make binary executable
	if err := b.run(fmt.Sprintf("chmod +x %s", binaryPath)); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Copy to /usr/local/bin
	if err := b.run(fmt.Sprintf("sudo cp %s %s", binaryPath, destBin)); err != nil {
		return fmt.Errorf("failed to copy to /usr/local/bin: %w", err)
	}

	if err := b.run(fmt.Sprintf("sudo mkdir -p %s", shareDir)); err != nil {
		return fmt.Errorf("failed to create share directory: %w", err)
	}
	if err := b.run(fmt.Sprintf("sudo cp %s %s", binaryPath, shareDir)); err != nil {
		return fmt.Errorf("failed to copy to share directory: %w", err)
	}

	fmt.Printf("Installed as '%s'\n", binName)
	return nil
}
