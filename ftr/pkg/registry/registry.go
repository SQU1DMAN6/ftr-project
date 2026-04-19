package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PackageInfo describes an installed package
type PackageInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version,omitempty"`
	Source      string    `json:"source,omitempty"` // user/repo
	Description string    `json:"description,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	InstallPath string    `json:"install_path,omitempty"`
	BinaryPath  string    `json:"binary_path,omitempty"`
}

type registryData struct {
	Packages []PackageInfo `json:"packages"`
}

// default registry locations
var (
	systemRegistry = "/var/lib/ftr/registry.json"
	userRegistry   = "~/.local/share/ftr/registry.json"
)

// ResolvePath returns the first writable registry path
func ResolvePath() (string, error) {
	// prefer system path
	dir := filepath.Dir(systemRegistry)
	if err := os.MkdirAll(dir, 0755); err == nil {
		// try to create file if not exists
		f, err := os.OpenFile(systemRegistry, os.O_RDWR|os.O_CREATE, 0644)
		if err == nil {
			f.Close()
			return systemRegistry, nil
		}
	}

	// fallback to user path
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home dir: %w", err)
	}
	userPath := filepath.Join(home, ".local", "share", "ftr", "registry.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create user registry dir: %w", err)
	}
	f, err := os.OpenFile(userPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create registry file: %w", err)
	}
	f.Close()
	return userPath, nil
}

func load() (*registryData, string, error) {
	path, err := ResolvePath()
	if err != nil {
		return nil, "", err
	}
	data := &registryData{}
	f, err := os.Open(path)
	if err != nil {
		return nil, path, err
	}
	defer f.Close()
	stat, _ := f.Stat()
	if stat.Size() == 0 {
		return data, path, nil
	}
	dec := json.NewDecoder(f)
	if err := dec.Decode(data); err != nil {
		return nil, path, err
	}
	return data, path, nil
}

func save(data *registryData, path string) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}

// Register adds or updates a package entry
func Register(pkg PackageInfo) error {
	data, path, err := load()
	if err != nil {
		return err
	}
	// replace if exists
	for i, p := range data.Packages {
		if p.Name == pkg.Name {
			pkg.InstalledAt = time.Now()
			data.Packages[i] = pkg
			return save(data, path)
		}
	}
	pkg.InstalledAt = time.Now()
	data.Packages = append(data.Packages, pkg)
	return save(data, path)
}

// Unregister removes a package entry by name
func Unregister(name string) error {
	data, path, err := load()
	if err != nil {
		return err
	}
	for i, p := range data.Packages {
		if p.Name == name {
			data.Packages = append(data.Packages[:i], data.Packages[i+1:]...)
			return save(data, path)
		}
	}
	return errors.New("package not found")
}

// Find returns a package by name
func Find(name string) (*PackageInfo, error) {
	data, _, err := load()
	if err != nil {
		return nil, err
	}
	for _, p := range data.Packages {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, errors.New("not found")
}

// List returns all registered packages
func List() ([]PackageInfo, error) {
	data, _, err := load()
	if err != nil {
		return nil, err
	}
	return data.Packages, nil
}
