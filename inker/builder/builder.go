package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrPrivilegedRequired indicates that the detected build requires privileged actions
// to be executed later by the caller (so the caller can prompt the user for a password
// or otherwise decide how to obtain elevation).
var ErrPrivilegedRequired = errors.New("privileged actions required")

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

	if requiresPriv {
		// Collect privileged operations to be executed later via the privileged runner.
		// This ensures that when elevated privileges are required the runner will
		// perform a GUI auth prompt on macOS (osascript) or a polkit dialog on Linux (pkexec),
		// instead of failing or prompting on the terminal.
		b.privilegedRunner.Add(command, args...)
		return nil
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
		// On macOS, many installers assume /usr/share which is protected by SIP.
		// Create a MAC-specific copy that rewrites /usr/share -> /usr/local/share
		// when possible so installations succeed without hitting SIP.
		if runtime.GOOS == "darwin" {
			origPath := filepath.Join(b.WorkDir, "install.sh")
			data, rerr := os.ReadFile(origPath)
			if rerr == nil {
				modified := strings.ReplaceAll(string(data), "/usr/share", "/usr/local/share")
				newName := "install.mac.sh"
				newPath := filepath.Join(b.WorkDir, newName)
				if werr := os.WriteFile(newPath, []byte(modified), 0755); werr == nil {
					b.privilegedRunner.Add("chmod", "+x", newPath)
					b.privilegedRunner.Add("sh", "-c", newPath)
					return "", ErrPrivilegedRequired
				}
				// If writing the modified script fails, fall back to original
			}
		}
		// Default: collect privileged steps and return a sentinel error to allow the
		// caller to prompt the user for credentials (or choose another
		// elevation method) before executing the actions. Use absolute paths
		// so the privileged runner can find the files regardless of its cwd.
		orig := filepath.Join(b.WorkDir, "install.sh")
		b.privilegedRunner.Add("chmod", "+x", orig)
		b.privilegedRunner.Add("sh", "-c", orig)
		return "", ErrPrivilegedRequired
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
	shareDir := filepath.Join("/usr/local/share", b.RepoName)

	// Make binary executable
	if err := b.run("sudo", "chmod", "+x", binaryPath); err != nil {
		return fmt.Errorf("changing permissions failed: %w", err)
	}

	if err := b.run("sudo", "cp", binaryPath, destBin); err != nil {
		log.Printf("warning: failed to copy binary to %s: %v", destBin, err)
	}

	// For macOS, avoid writing into system-wide share directories which may be
	// restricted by SIP. Use a per-user Application Support folder instead so
	// installations remain stable and do not require system-level writes when
	// possible.
	if runtime.GOOS == "darwin" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return fmt.Errorf("failed to determine home directory: %w", herr)
		}
		userShare := filepath.Join(home, "Library", "Application Support", b.RepoName)
		if err := os.MkdirAll(userShare, 0755); err != nil {
			return fmt.Errorf("failed to create user share directory: %w", err)
		}
		dest := filepath.Join(userShare, binName)
		if err := copyFile(binaryPath, dest); err != nil {
			return fmt.Errorf("failed to copy binary to user share dir: %w", err)
		}
	} else {
		// Non-macOS: attempt to create and copy into system-wide share directory
		if err := b.run("sudo", "mkdir", "-p", shareDir); err != nil {
			log.Printf("warning: could not create %s: %v", shareDir, err)
		}
		if err := b.run("sudo", "cp", binaryPath, shareDir); err != nil {
			log.Printf("warning: could not copy to %s: %v", shareDir, err)
		}
	}

	// Now, execute all collected privileged commands (if any).
	if err := b.privilegedRunner.Execute(); err != nil {
		// If privileged execution failed, attempt a user-local fallback for the
		// share directory so installation can still succeed without system write
		// access.
		log.Printf("privileged execution failed: %v", err)
		home, herr := os.UserHomeDir()
		if herr != nil {
			return fmt.Errorf("installation failed and could not determine fallback location: %w", err)
		}

		fallbackShare := filepath.Join(home, ".local", "share", b.RepoName)
		if err2 := os.MkdirAll(fallbackShare, 0755); err2 != nil {
			return fmt.Errorf("installation failed: %w; fallback creation also failed: %v", err, err2)
		}
		dest := filepath.Join(fallbackShare, binName)
		if err3 := copyFile(binaryPath, dest); err3 != nil {
			return fmt.Errorf("installation failed: %w; fallback copy also failed: %v", err, err3)
		}
		log.Printf("privileged actions failed but binary copied to fallback: %s", dest)
	}

	log.Printf("Installed as '%s'", binName)
	return nil
}

// copyFile copies a file from src to dst preserving executable bit when possible.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	// Try to copy mode from source file
	if fi, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, fi.Mode())
	}
	return nil
}

// ExecutePrivileged runs any commands collected by the PrivilegedRunner.
// This allows the caller to collect commands during detection/build and
// then perform elevation after prompting the user for credentials.
func (b *Builder) ExecutePrivileged() error {
	return b.privilegedRunner.Execute()
}
