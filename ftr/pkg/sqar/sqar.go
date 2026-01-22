package sqar

import (
	"os"
	"os/exec"
)

// findSqarTool searches for an available 'sqar' tool.
// It checks the SQAR_TOOL env var, then searches PATH, then common locations.
func FindSqarTool() string {
	if v := os.Getenv("SQAR_TOOL"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}

	if p, err := exec.LookPath("sqar"); err == nil {
		return p
	}

	paths := []string{
		"/usr/local/bin/sqar",
		"/opt/homebrew/bin/sqar",
		"/usr/bin/sqar",
		"/bin/sqar",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
