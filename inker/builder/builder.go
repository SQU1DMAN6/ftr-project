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

var ErrPrivilegedRequired = errors.New("privileged actions required")

// ErrDelegatedToTerminal indicates privileged commands were delegated to the user's Terminal.app for interactive execution (macOS only).
var ErrDelegatedToTerminal = errors.New("delegated to terminal")

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
		// First, verify the provided password works with sudo by running a
		// small test command. This provides an earlier, clearer error when
		// authentication fails instead of running the full script.
		checkCmd := exec.Command("sudo", "-S", "-k", "-p", "", "sh", "-c", "echo SUDO_AUTH_OK")
		var checkOut, checkErr bytes.Buffer
		checkCmd.Stdout = &checkOut
		checkCmd.Stderr = &checkErr
		checkCmd.Stdin = strings.NewReader(pr.Password + "\n")
		if err := checkCmd.Run(); err != nil || !strings.Contains(checkOut.String(), "SUDO_AUTH_OK") {
			out := strings.TrimSpace(checkOut.String() + "\n" + checkErr.String())
			if err != nil {
				return fmt.Errorf("sudo authentication failed: %w\nOutput:\n%s", err, out)
			}
			return fmt.Errorf("sudo authentication failed: Output:\n%s", out)
		}

		cmd = exec.Command("sudo", "-S", "sh", "-c", script)
		cmd.Stdin = strings.NewReader(pr.Password + "\n")
	} else {
		switch runtime.GOOS {
		case "darwin":
			// On macOS, delegate the privileged script to Terminal so the user
			// sees the install running in the standard terminal environment
			// (and can enter sudo password interactively). Create a temporary
			// shell script and instruct Terminal.app to run it. Because the
			// actual commands will run asynchronously in Terminal, return a
			// sentinel error so the caller knows execution was delegated.
			tmpFile, merr := os.CreateTemp("", "ftr-priv-*.sh")
			if merr != nil {
				return fmt.Errorf("failed to create temp script: %w", merr)
			}
			tmpPath := tmpFile.Name()
			// Add an EXIT trap to close the Terminal window only on success.
			// This avoids hiding failures by closing the window when the
			// installer fails; users can still see errors in Terminal.
			trap := "trap 'status=$?; if [ \"$status\" -eq 0 ]; then /usr/bin/osascript -e \"tell application \\\"Terminal\\\" to close front window\"; fi; exit $status' EXIT\n"
			if _, werr := tmpFile.WriteString("#!/bin/sh\nset -e\n" + trap + script); werr != nil {
				tmpFile.Close()
				return fmt.Errorf("failed to write temp script: %w", werr)
			}
			tmpFile.Close()
			if chmodErr := os.Chmod(tmpPath, 0755); chmodErr != nil {
				return fmt.Errorf("failed to mark temp script executable: %w", chmodErr)
			}

			// Build AppleScript to open Terminal and run the script. Use /bin/sh
			// to ensure consistent shell behavior for GUI-launched terminals.
			// Escape double quotes in path just in case.
			safePath := strings.ReplaceAll(tmpPath, "\"", "\\\"")
			// Run the script with sudo so the terminal prompts for the user's
			// password and the installer runs with elevated privileges.
			termCmd := fmt.Sprintf("/bin/sh '%s'", safePath)
			appleScript := fmt.Sprintf("tell application \"Terminal\" to do script \"%s\"", termCmd)
			cmd = exec.Command("osascript", "-e", appleScript)

			// Run the osascript command to open Terminal; do not wait for the
			// script inside Terminal to finish — delegate and inform caller.
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to launch Terminal: %w\nOutput:\n%s", err, stderr.String())
			}
			return ErrDelegatedToTerminal
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
		// Resolve executable path to handle limited PATH when launched from GUI bundles
		cmdPath, perr := findExecutable(command)
		if perr != nil {
			return perr
		}
		cmd := exec.Command(cmdPath, args...)
		// If invoking `go`, ensure GOROOT is set when the binary is a trimmed
		// Homebrew shim. Try common libexec locations used by Homebrew and
		// system installs and set GOROOT in the command environment if found.
		if command == "go" {
			env := os.Environ()
			// Common possible GOROOT locations
			candidates := []string{
				"/opt/homebrew/opt/go/libexec",
				"/usr/local/opt/go/libexec",
				"/usr/local/go",
			}
			// Also try a libexec next to the go binary (two levels up)/libexec
			binDir := filepath.Dir(cmdPath)
			candidates = append([]string{filepath.Join(filepath.Dir(binDir), "opt", "go", "libexec")}, candidates...)
			for _, g := range candidates {
				if g == "" {
					continue
				}
				if fi, err := os.Stat(g); err == nil && fi.IsDir() {
					env = append(env, "GOROOT="+g)
					cmd.Env = env
					log.Printf("Setting GOROOT=%s for go binary", g)
					break
				}
			}
		}
		cmd.Dir = b.WorkDir

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %w\nOutput:\n%s", err, strings.TrimSpace(out.String()))
		}
		return nil
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

	// Resolve executable path to handle limited PATH when launched from GUI bundles
	cmdPath, perr := findExecutable(command)
	if perr != nil {
		return perr
	}
	cmd := exec.Command(cmdPath, args...)
	// If invoking `go`, ensure GOROOT is set similarly for non-root runs.
	if command == "go" {
		env := os.Environ()
		candidates := []string{
			"/opt/homebrew/opt/go/libexec",
			"/usr/local/opt/go/libexec",
			"/usr/local/go",
		}
		binDir := filepath.Dir(cmdPath)
		candidates = append([]string{filepath.Join(filepath.Dir(binDir), "opt", "go", "libexec")}, candidates...)
		for _, g := range candidates {
			if g == "" {
				continue
			}
			if fi, err := os.Stat(g); err == nil && fi.IsDir() {
				env = append(env, "GOROOT="+g)
				cmd.Env = env
				log.Printf("Setting GOROOT=%s for go binary", g)
				break
			}
		}
	}
	cmd.Dir = b.WorkDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w\nOutput:\n%s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

func (b *Builder) DetectAndBuild() (string, error) {
	// Check for install.sh first
	if _, err := os.Stat(filepath.Join(b.WorkDir, "install.sh")); err == nil {
		log.Println("install.sh found. Running and skipping default installer protocol...")
		// Collect privileged steps and return a sentinel error to allow the
		// caller to prompt the user for credentials (or choose another
		// elevation method) before executing the actions. Use absolute paths
		// so the privileged runner can find the files regardless of its cwd.
		orig := filepath.Join(b.WorkDir, "install.sh")
		// Ensure the installer runs with the package's workdir as the current
		// directory; some installers expect to be executed from that location.
		b.privilegedRunner.Add("chmod", "+x", orig)
		b.privilegedRunner.Add("sh", "-c", fmt.Sprintf("cd '%s' && sh '%s'", b.WorkDir, orig))
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
		// Ensure the required build tool exists and is visible to GUI-launched apps.
		if _, err := findExecutable("squ1d++"); err != nil {
			return "", fmt.Errorf("required build tool 'squ1d++' not found: %w", err)
		}

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

	// Attempt to create and copy into system-wide share directory
	if err := b.run("sudo", "mkdir", "-p", shareDir); err != nil {
		log.Printf("warning: could not create %s: %v", shareDir, err)
	}
	if err := b.run("sudo", "cp", binaryPath, shareDir); err != nil {
		log.Printf("warning: could not copy to %s: %v", shareDir, err)
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

// findExecutable attempts to locate an executable. It first uses exec.LookPath
// (which respects the current PATH). If not found, it searches common macOS
// and Homebrew locations so GUI-launched apps (with a limited PATH) can still
// find developer tools like `go`.
func findExecutable(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	// Check common locations on macOS/Homebrew
	if runtime.GOOS == "darwin" {
		candidates := []string{
			filepath.Join("/opt/homebrew/bin", name),
			filepath.Join("/usr/local/bin", name),
			filepath.Join("/usr/local/go/bin", name),
			filepath.Join("/usr/bin", name),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}

	// As a last resort, return an informative error
	return "", fmt.Errorf("executable %q not found in PATH; please install it or set PATH for GUI apps", name)
}
