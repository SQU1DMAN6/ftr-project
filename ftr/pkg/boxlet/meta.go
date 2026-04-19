package boxlet

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MetaKeyValue represents metadata for a boxlet
type MetaKeyValue map[string]string

// WriteMeta writes metadata key=value pairs to /BUILD/Meta.config under dir
func WriteMeta(dir string, meta MetaKeyValue) error {
	buildDir := filepath.Join(dir, "BUILD")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create BUILD directory: %w", err)
	}

	filePath := filepath.Join(buildDir, "Meta.config")
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for k, v := range meta {
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, strings.TrimSpace(v)); err != nil {
			return fmt.Errorf("failed to write meta: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush meta file: %w", err)
	}
	return nil
}

// ReadMeta reads /BUILD/Meta.config and returns key/value pairs
func ReadMeta(dir string) (MetaKeyValue, error) {
	// Prefer the new fsdlbuild.ftr file but fall back to legacy Meta.config for compatibility
	candidates := []string{
		filepath.Join(dir, "BUILD", "fsdlbuild.ftr"),
		filepath.Join(dir, "BUILD", "Meta.config"),
	}

	var f *os.File
	var err error
	for _, p := range candidates {
		f, err = os.Open(p)
		if err == nil {
			break
		}
	}
	if f == nil {
		return nil, fmt.Errorf("failed to open meta file: %w", err)
	}
	defer f.Close()

	meta := MetaKeyValue{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 1 {
			// Handle flags without values (e.g., INSTALL_AS_CLI) by setting them to "true"
			meta[strings.TrimSpace(parts[0])] = "true"
		} else if len(parts) == 2 {
			meta[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read meta file: %w", err)
	}
	return meta, nil
}
