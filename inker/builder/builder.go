package builder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// run executes a command and returns an error if any
func (b *Builder) run(cmd string) error {
	command := exec.Command("sh", "-c", cmd)
	command.Dir = b.WorkDir
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func (b *Builder) DetectAndBuild() (string, error) {
	// Check for install.sh first
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		log.Println("install.sh found. Running and skipping default installer protocol...")
		if err := b.run("chmod +x install.sh && ./install.sh"); err != nil {
			return "", fmt.Errorf("installation failed: %w", err)
		}
		return "", nil
	}

	// Then check for Makefile
	if _, err := os.Stat(filepath.Join(b.WorkDir, "Makefile")); err == nil {
		log.Println("Makefile found. Running make...")
		if err := b.run("make"); err != nil {
			return "", fmt.Errorf("installation failed: %w", err)
		}
		return "", nil
	}

	// Check for main.py
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.py")); err == nil {
		log.Println("Found Python app. Building with pyinstaller...")
		if err := b.run("python3 -m venv venv"); err != nil {
			return "", fmt.Errorf("failed to initialise temporary build environemnt")
		} else {
			log.Println("Created build environment")
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
		log.Println("Found Go app. Building...")
		if err := b.run(fmt.Sprintf("go build -o %s .", b.RepoName)); err != nil {
			return "", fmt.Errorf("build failed: %w", err)
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
	shareDir := filepath.Join("/usr/share", b.RepoName)

	// Make binary executable
	if err := b.run(fmt.Sprintf("chmod +x %s", binaryPath)); err != nil {
		return fmt.Errorf("changing permissions failed: %w", err)
	}

	if err := b.run(fmt.Sprintf("sudo cp %s %s", binaryPath, destBin)); err != nil {
		return fmt.Errorf("failed to copy app file to /usr/local/bin: %w", err)
	}

	// Create and copy app file to share directory if kernel is not darwin
	if runtime.GOOS != "darwin" {
		if err := b.run(fmt.Sprintf("sudo mkdir -p %s", shareDir)); err != nil {
			return fmt.Errorf("failed to create share directory: %w", err)
		}
		if err := b.run(fmt.Sprintf("sudo cp %s %s", binaryPath, shareDir)); err != nil {
			return fmt.Errorf("failed to copy app file to share directory: %w", err)
		}
	}

	log.Printf("Installed as '%s'", binName)
	return nil
}
