package builder

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// Builder handles the detection and building of different project types
type Builder struct {
	RepoName         string
	WorkDir          string
	privilegedRunner *PrivilegedRunner
	Password         string
}

// PrivilegedRunner collects and runs commands requiring elevated privileges.
type PrivilegedRunner struct {
	commands []string
	Password string
	UseSudo  bool
}

// NewPrivilegedRunner creates a new runner.
func NewPrivilegedRunner() *PrivilegedRunner {
	return &PrivilegedRunner{
		commands: []string{},
	}
}

// Add adds a command to be run with privileges.
func (pr *PrivilegedRunner) Add(command string, args ...string) {
	// Shell-escape arguments to handle spaces and special characters safely.
	var cmdParts []string
	cmdParts = append(cmdParts, command)
	for _, arg := range args {
		cmdParts = append(cmdParts, fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
	}
	pr.commands = append(pr.commands, strings.Join(cmdParts, " "))
}

// Execute runs all collected commands using a single pkexec prompt.
func (pr *PrivilegedRunner) Execute() error {
	if len(pr.commands) == 0 {
		return nil // Nothing to do
	}

	script := "#!/bin/sh\nset -e\n"
	for _, cmd := range pr.commands {
		script += cmd + "\n"
	}

	var cmd *exec.Cmd
	if pr.UseSudo {
		cmd = exec.Command("sudo", "-S", "sh", "-c", script)
		cmd.Stdin = strings.NewReader(pr.Password + "\n")
	} else {
		switch runtime.GOOS {
		case "darwin":
			escapedScript := strings.ReplaceAll(script, "\\", "\\\\")
			escapedScript = strings.ReplaceAll(escapedScript, "\"", "\\\"")
			appleScript := fmt.Sprintf("do shell script \"%s\" with administrator privileges", escapedScript)
			cmd = exec.Command("osascript", "-e", appleScript)
		default:
			cmd = exec.Command("pkexec", "sh", "-c", script)
		}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("privileged execution failed: %w\nOutput:\n%s", err, stderr.String())
	}
	return nil
}

// New creates a new Builder instance
func New(repoName, workDir string) *Builder {
	return &Builder{
		RepoName:         repoName,
		WorkDir:          workDir,
		privilegedRunner: NewPrivilegedRunner(),
	}
}

func (b *Builder) SetPassword(password string) {
	b.Password = password
	b.privilegedRunner.Password = password
	b.privilegedRunner.UseSudo = true
}

func (b *Builder) run(command string, args ...string) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("could not get current user: %w", err)
	}

	if currentUser.Uid == "0" {
		cmd := exec.Command(command, args...)
		cmd.Dir = b.WorkDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if command == "sudo" {
		if len(args) == 0 {
			return fmt.Errorf("sudo used with no command")
		}
		b.privilegedRunner.Add(args[0], args[1:]...)
		return nil
	}

	privileged := map[string]bool{
		"cp":       true,
		"mv":       true,
		"rm":       true,
		"mkdir":    true,
		"chmod":    true,
		"chown":    true,
		"xattr":    true,
		"codesign": true,
		"echo":     true,
		"touch":    true,
		"chflags":  true,
	}

	requiresPriv := privileged[command]

	if runtime.GOOS == "darwin" && requiresPriv {
		b.privilegedRunner.Add(command, args...)
		return nil
	}

	if requiresPriv {
		return fmt.Errorf(
			"attempted privileged operation without sudo: %s %v",
			command, args,
		)
	}

	cmd := exec.Command(command, args...)
	cmd.Dir = b.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Builder) DetectAndBuild() (string, error) {
	// Check for install.sh first
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		log.Println("install.sh found. Running and skipping default installer protocol...")
		if err := b.run("sh", "-c", "chmod +x install.sh && ./install.sh"); err != nil {
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
		if err := b.run("python3", "-m", "venv", "venv"); err != nil {
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

		// Using "sudo" here will trigger the pkexec replacement in run()
		if err := b.run("sudo", "pyinstaller", "--noconsole", "--onefile", "main.py", "--name", b.RepoName, importFlags); err != nil {
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
		if err := b.run("go", "build", "-o", b.RepoName, "."); err != nil {
			return "", fmt.Errorf("build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.cpp
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.cpp")); err == nil {
		fmt.Println("Detected C++ app. Building with g++...")
		if err := b.run("g++", "main.cpp", "-o", b.RepoName); err != nil {
			return "", fmt.Errorf("g++ build failed: %w", err)
		}
		return filepath.Join(b.WorkDir, b.RepoName), nil
	}

	// Check for main.sqd
	if _, err := os.Stat(filepath.Join(b.WorkDir, "main.sqd")); err == nil {
		fmt.Println("Detected SQU1D app. Building with squ1d++...")
		if err := b.run("squ1d++", "-B", "main.sqd", "-o", b.RepoName); err != nil {
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
	if err := b.run("sudo", "chmod", "+x", binaryPath); err != nil {
		return fmt.Errorf("changing permissions failed: %w", err)
	}

	b.run("sudo", "cp", binaryPath, destBin)

	// Create and copy app file to share directory if kernel is not darwin
	if runtime.GOOS != "darwin" {
		b.run("sudo", "mkdir", "-p", shareDir)
		b.run("sudo", "cp", binaryPath, shareDir)
	}

	// Now, execute all collected privileged commands.
	if err := b.privilegedRunner.Execute(); err != nil {
		return fmt.Errorf("installation script failed: %w", err)
	}

	log.Printf("Installed as '%s'", binName)
	return nil
}
