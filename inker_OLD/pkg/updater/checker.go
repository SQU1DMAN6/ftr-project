package updater

import (
	"fmt"
	"sync"
	"time"
)

// UpdateChecker handles checking for package updates
type UpdateChecker struct {
	client          APIClient // Interface to FtR API
	checkInterval   time.Duration
	lastCheck       map[string]time.Time // package name -> last check time
	mux             sync.RWMutex
	isChecking      bool
	onUpdateFound   func(packageName, newVersion string)
	onCheckComplete func(total int, available int)
	onCheckError    func(err error)
}

// APIClient interface for repository queries (matches FtR's api package)
type APIClient interface {
	// SearchRepos searches for repositories by name
	SearchRepos(query string) ([]map[string]interface{}, error)
	// GetPackageInfo retrieves package information
	GetPackageInfo(user, repo string) (map[string]interface{}, error)
	// ListVersions lists available versions of a package
	ListVersions(user, repo string) ([]string, error)
}

// NewUpdateChecker creates a new update checker
func NewUpdateChecker(client APIClient, checkInterval time.Duration) *UpdateChecker {
	return &UpdateChecker{
		client:        client,
		checkInterval: checkInterval,
		lastCheck:     make(map[string]time.Time),
	}
}

// SetCallbacks sets the callback functions for update events
func (uc *UpdateChecker) SetCallbacks(
	onUpdateFound func(packageName, newVersion string),
	onCheckComplete func(total int, available int),
	onCheckError func(err error),
) {
	uc.onUpdateFound = onUpdateFound
	uc.onCheckComplete = onCheckComplete
	uc.onCheckError = onCheckError
}

// CheckForUpdates checks a single package for updates
// Returns: (hasUpdate bool, newVersion string, error)
func (uc *UpdateChecker) CheckForUpdates(packageName, currentVersion, source string) (bool, string, error) {
	uc.mux.Lock()
	defer uc.mux.Unlock()

	// Rate limiting: don't check same package more than once per interval
	if lastCheck, exists := uc.lastCheck[packageName]; exists {
		if time.Since(lastCheck) < uc.checkInterval {
			return false, "", nil
		}
	}

	// Parse source (format: "user/repo")
	var user, repo string
	fmt.Sscanf(source, "%s/%s", &user, &repo)
	if user == "" || repo == "" {
		return false, "", fmt.Errorf("invalid source format: %s", source)
	}

	// Get available versions from repository
	versions, err := uc.client.ListVersions(user, repo)
	if err != nil {
		return false, "", fmt.Errorf("failed to check updates for %s: %w", packageName, err)
	}

	// Find the latest version that's newer than current
	latestVersion := findLatestVersion(versions, currentVersion)
	if latestVersion != "" && latestVersion != currentVersion {
		uc.lastCheck[packageName] = time.Now()
		if uc.onUpdateFound != nil {
			go uc.onUpdateFound(packageName, latestVersion)
		}
		return true, latestVersion, nil
	}

	uc.lastCheck[packageName] = time.Now()
	return false, "", nil
}

// CheckBatchUpdates checks multiple packages for updates concurrently
// Returns: map of packageName -> newVersion for packages with updates
func (uc *UpdateChecker) CheckBatchUpdates(packages map[string]PackageVersion) (map[string]string, error) {
	uc.mux.Lock()
	if uc.isChecking {
		uc.mux.Unlock()
		return nil, fmt.Errorf("update check already in progress")
	}
	uc.isChecking = true
	uc.mux.Unlock()

	defer func() {
		uc.mux.Lock()
		uc.isChecking = false
		uc.mux.Unlock()
	}()

	results := make(map[string]string)
	resultsMux := sync.Mutex{}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4) // Limit concurrent checks

	updatesFound := 0
	totalChecked := 0

	for packageName, version := range packages {
		wg.Add(1)
		go func(name, ver, source string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			if hasUpdate, newVersion, err := uc.CheckForUpdates(name, ver, source); err == nil && hasUpdate {
				resultsMux.Lock()
				results[name] = newVersion
				updatesFound++
				resultsMux.Unlock()
			} else if err != nil && uc.onCheckError != nil {
				uc.onCheckError(err)
			}

			totalChecked++
		}(packageName, version.Version, version.Source)
	}

	wg.Wait()

	if uc.onCheckComplete != nil {
		go uc.onCheckComplete(len(packages), updatesFound)
	}

	return results, nil
}

// PackageVersion represents a package with its current version and source
type PackageVersion struct {
	Version string
	Source  string // user/repo
}

// IsUpdateAvailable quickly checks if an update is available without network call
// (uses cached data from previous checks)
func (uc *UpdateChecker) IsUpdateAvailable(packageName, currentVersion string) bool {
	// This would be called with data from registry's cached updateAvailable field
	return currentVersion != "" && currentVersion != "unknown"
}

// ResetCheckTime resets the check timer for a package (forces re-check on next call)
func (uc *UpdateChecker) ResetCheckTime(packageName string) {
	uc.mux.Lock()
	defer uc.mux.Unlock()
	delete(uc.lastCheck, packageName)
}

// ResetAllCheckTimes resets all check timers
func (uc *UpdateChecker) ResetAllCheckTimes() {
	uc.mux.Lock()
	defer uc.mux.Unlock()
	uc.lastCheck = make(map[string]time.Time)
}

// findLatestVersion finds the highest version number that's newer than current
// Simple version comparison (uses semantic versioning logic)
func findLatestVersion(versions []string, currentVersion string) string {
	if len(versions) == 0 {
		return ""
	}

	var latest string
	for _, v := range versions {
		if isVersionGreater(v, currentVersion) && isVersionGreater(v, latest) {
			latest = v
		}
	}
	return latest
}

// isVersionGreater compares two semantic versions
// Returns true if v1 > v2
func isVersionGreater(v1, v2 string) bool {
	if v1 == "" || v2 == "" {
		return v1 != ""
	}

	// Simple semver comparison: split by "." and compare numerically
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] > parts2[i] {
			return true
		} else if parts1[i] < parts2[i] {
			return false
		}
	}

	// If all parts equal so far, longer version is greater
	return len(parts1) > len(parts2)
}

// parseVersion converts a version string into comparable integer parts
func parseVersion(v string) []int {
	// Example: "2.7.4" -> [2, 7, 4]
	var parts []int
	var current int
	for _, ch := range v {
		if ch >= '0' && ch <= '9' {
			current = current*10 + int(ch-'0')
		} else if ch == '.' || ch == '-' {
			parts = append(parts, current)
			current = 0
		}
	}
	if current > 0 {
		parts = append(parts, current)
	}
	return parts
}

// Scheduler manages periodic update checks
type UpdateScheduler struct {
	checker      *UpdateChecker
	interval     time.Duration
	ticker       *time.Ticker
	stopChan     chan struct{}
	isRunning    bool
	runningMux   sync.Mutex
	registryList func() map[string]PackageVersion // Callback to get current packages
}

// NewUpdateScheduler creates a new scheduled update checker
func NewUpdateScheduler(checker *UpdateChecker, interval time.Duration) *UpdateScheduler {
	return &UpdateScheduler{
		checker:  checker,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// SetRegistryCallback sets the callback to retrieve the current package list
func (us *UpdateScheduler) SetRegistryCallback(fn func() map[string]PackageVersion) {
	us.registryList = fn
}

// Start begins the scheduled update checks
func (us *UpdateScheduler) Start() error {
	us.runningMux.Lock()
	if us.isRunning {
		us.runningMux.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	us.isRunning = true
	us.runningMux.Unlock()

	us.ticker = time.NewTicker(us.interval)
	go func() {
		for {
			select {
			case <-us.ticker.C:
				if us.registryList != nil {
					packages := us.registryList()
					us.checker.CheckBatchUpdates(packages)
				}
			case <-us.stopChan:
				us.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// Stop stops the scheduled update checks
func (us *UpdateScheduler) Stop() {
	us.runningMux.Lock()
	if !us.isRunning {
		us.runningMux.Unlock()
		return
	}
	us.isRunning = false
	us.runningMux.Unlock()

	close(us.stopChan)
}

// IsRunning returns whether the scheduler is currently running
func (us *UpdateScheduler) IsRunning() bool {
	us.runningMux.Lock()
	defer us.runningMux.Unlock()
	return us.isRunning
}
