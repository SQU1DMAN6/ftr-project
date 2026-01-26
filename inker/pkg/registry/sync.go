package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PackageInfo describes an installed package (mirrors FtR's structure)
type PackageInfo struct {
	Name               string    `json:"name"`
	Version            string    `json:"version,omitempty"`
	Source             string    `json:"source,omitempty"` // user/repo
	InstalledAt        time.Time `json:"installed_at"`
	BinaryPath         string    `json:"binary_path,omitempty"`
	UpdateAvailable    string    `json:"update_available,omitempty"`
	LastUpdateChecked  time.Time `json:"last_update_checked,omitempty"`
	UpdateCheckError   string    `json:"update_check_error,omitempty"`
	Architecture       string    `json:"architecture,omitempty"`
	OS                 string    `json:"os,omitempty"`
	Dependencies       []string  `json:"dependencies,omitempty"`
	Size               int64     `json:"size,omitempty"`
	Description        string    `json:"description,omitempty"`
	Homepage           string    `json:"homepage,omitempty"`
	InstallationMethod string    `json:"installation_method,omitempty"` // install.sh, binary, etc.
}

type registryData struct {
	Packages        []PackageInfo `json:"packages"`
	LastSync        time.Time     `json:"last_sync"`
	SyncError       string        `json:"sync_error,omitempty"`
	InkerVersion    string        `json:"inker_version"`
	RegistryVersion string        `json:"registry_version"`
}

// Registry paths (mirrors FtR paths for compatibility)
var (
	systemRegistryPath = "/var/lib/ftr/registry.json"
	userRegistryPath   = "~/.local/share/ftr/registry.json"
	inkerCacheFile     = "~/.cache/inker/registry.json"
)

// Registry manages the local package registry
type Registry struct {
	path string
	data *registryData
	mux  chan struct{} // Simple mutex for thread-safe access
}

// NewRegistry creates a new registry instance or loads existing one
func NewRegistry() (*Registry, error) {
	reg := &Registry{
		mux: make(chan struct{}, 1),
	}
	reg.mux <- struct{}{}

	// Determine registry path (check FtR paths first, then create Inker cache)
	path, err := resolveRegistryPath()
	if err != nil {
		// Fall back to Inker cache
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".cache", "inker", "registry.json")
		os.MkdirAll(filepath.Dir(path), 0755)
	}

	reg.path = path
	if err := reg.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if reg.data == nil {
		reg.data = &registryData{
			Packages:        []PackageInfo{},
			LastSync:        time.Now(),
			RegistryVersion: "2.0",
		}
	}

	return reg, nil
}

// resolveRegistryPath returns the first accessible registry path
func resolveRegistryPath() (string, error) {
	// Try system registry first
	if _, err := os.Stat(systemRegistryPath); err == nil {
		return systemRegistryPath, nil
	}

	// Try user registry
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".local", "share", "ftr", "registry.json")
		if _, err := os.Stat(userPath); err == nil {
			return userPath, nil
		}
	}

	return "", fmt.Errorf("no FtR registry found")
}

// load reads the registry from disk
func (r *Registry) load() error {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	f, err := os.Open(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	if stat.Size() == 0 {
		r.data = &registryData{
			Packages:        []PackageInfo{},
			LastSync:        time.Now(),
			RegistryVersion: "2.0",
		}
		return nil
	}

	dec := json.NewDecoder(f)
	if err := dec.Decode(&r.data); err != nil {
		return fmt.Errorf("failed to decode registry: %w", err)
	}

	return nil
}

// Save writes the registry to disk
func (r *Registry) Save() error {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data == nil {
		r.data = &registryData{
			Packages:        []PackageInfo{},
			LastSync:        time.Now(),
			RegistryVersion: "2.0",
		}
	}

	r.data.LastSync = time.Now()

	os.MkdirAll(filepath.Dir(r.path), 0755)

	f, err := os.Create(r.path)
	if err != nil {
		return fmt.Errorf("failed to create registry file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r.data); err != nil {
		return fmt.Errorf("failed to encode registry: %w", err)
	}

	return nil
}

// GetAllPackages returns all installed packages
func (r *Registry) GetAllPackages() []PackageInfo {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data == nil {
		return []PackageInfo{}
	}

	// Return copy to prevent external modification
	packages := make([]PackageInfo, len(r.data.Packages))
	copy(packages, r.data.Packages)
	return packages
}

// GetPackage retrieves a specific package by name
func (r *Registry) GetPackage(name string) (*PackageInfo, error) {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data == nil {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	for i := range r.data.Packages {
		if r.data.Packages[i].Name == name {
			pkg := r.data.Packages[i]
			return &pkg, nil
		}
	}

	return nil, fmt.Errorf("package not found: %s", name)
}

// AddPackage adds or updates a package in the registry
func (r *Registry) AddPackage(pkg PackageInfo) error {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data == nil {
		r.data = &registryData{
			Packages:        []PackageInfo{},
			RegistryVersion: "2.0",
		}
	}

	// Check if package already exists
	for i := range r.data.Packages {
		if r.data.Packages[i].Name == pkg.Name {
			r.data.Packages[i] = pkg
			return nil
		}
	}

	// New package
	r.data.Packages = append(r.data.Packages, pkg)
	return nil
}

// RemovePackage removes a package from the registry
func (r *Registry) RemovePackage(name string) error {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data == nil {
		return fmt.Errorf("package not found: %s", name)
	}

	for i := range r.data.Packages {
		if r.data.Packages[i].Name == name {
			r.data.Packages = append(r.data.Packages[:i], r.data.Packages[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("package not found: %s", name)
}

// GetUpdatablePackages returns packages with available updates
func (r *Registry) GetUpdatablePackages() []PackageInfo {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	var updatable []PackageInfo
	for _, pkg := range r.data.Packages {
		if pkg.UpdateAvailable != "" && pkg.UpdateAvailable != pkg.Version {
			updatable = append(updatable, pkg)
		}
	}
	return updatable
}

// SetLastSync updates the last sync time
func (r *Registry) SetLastSync(t time.Time) {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data != nil {
		r.data.LastSync = t
		r.data.SyncError = ""
	}
}

// SetSyncError records a sync error
func (r *Registry) SetSyncError(errMsg string) {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data != nil {
		r.data.SyncError = errMsg
	}
}

// GetLastSync returns the last successful sync time
func (r *Registry) GetLastSync() time.Time {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	if r.data != nil {
		return r.data.LastSync
	}
	return time.Time{}
}

// Statistics about the registry
type RegistryStats struct {
	TotalPackages    int
	UpdatesAvailable int
	LastSync         time.Time
	SyncError        string
	RegistryVersion  string
}

// GetStats returns statistics about the registry
func (r *Registry) GetStats() RegistryStats {
	<-r.mux
	defer func() { r.mux <- struct{}{} }()

	stats := RegistryStats{
		RegistryVersion: "2.0",
	}

	if r.data == nil {
		return stats
	}

	stats.TotalPackages = len(r.data.Packages)
	stats.LastSync = r.data.LastSync
	stats.SyncError = r.data.SyncError

	for _, pkg := range r.data.Packages {
		if pkg.UpdateAvailable != "" && pkg.UpdateAvailable != pkg.Version {
			stats.UpdatesAvailable++
		}
	}

	return stats
}
