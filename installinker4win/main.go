package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows/registry"
)

const inkerURL = "https://quanthai.net/inker.zip"

type Logger struct {
	mu      sync.Mutex
	output  *widget.RichText
	logText strings.Builder
}

func NewLogger(output *widget.RichText) *Logger {
	return &Logger{
		output: output,
	}
}

func (l *Logger) Log(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("[%s] %s\n", timestamp, msg)
	l.logText.WriteString(formatted)
	// Update the RichText content by rebuilding with all text
	l.output.ParseMarkdown("```\n" + l.logText.String() + "```")
}

func (l *Logger) Logf(format string, args ...interface{}) {
	l.Log(fmt.Sprintf(format, args...))
}

func main() {
	a := app.NewWithID("com.ftr.installinker4win")
	w := a.NewWindow("Inker for Windows - Installer")

	output := widget.NewRichTextFromMarkdown("## Installation Log\n\nPress the Install button to begin.\n")
	outputScroll := container.NewScroll(output)
	outputScroll.SetMinSize(fyne.NewSize(680, 400))

	logger := NewLogger(output)

	var installBtn *widget.Button
	installBtn = widget.NewButtonWithIcon("Install Inker", theme.DownloadIcon(), func() {
		installBtn.Disable()
		go performInstall(logger, installBtn)
	})

	clearBtn := widget.NewButton("Clear Log", func() {
		output.ParseMarkdown("## Installation Log\n\n")
		logger.mu.Lock()
		logger.logText.Reset()
		logger.mu.Unlock()
	})

	buttonBar := container.NewHBox(
		installBtn,
		clearBtn,
		layout.NewSpacer(),
	)

	mainContent := container.NewBorder(
		nil,
		buttonBar,
		nil,
		nil,
		outputScroll,
	)

	w.SetContent(mainContent)
	w.ShowAndRun()
}

func findInkerInstallPath(logger *Logger) (string, bool) {
	// Check user registry first
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\FtR\Packages\inker`, registry.READ)
	if err == nil {
		defer key.Close()
		// Prefer BinaryPath if it exists, fall back to InstallPath
		path, _, err := key.GetStringValue("BinaryPath")
		if err == nil && path != "" {
			logger.Logf("Found existing user installation from registry (BinaryPath): %s", path)
			return path, true
		}
		path, _, err = key.GetStringValue("InstallPath")
		if err == nil && path != "" {
			logger.Logf("Found existing user installation from registry (InstallPath): %s", path)
			return path, true
		}
	}

	// Check system registry
	key, err = registry.OpenKey(registry.LOCAL_MACHINE, `Software\FtR\Packages\inker`, registry.READ)
	if err == nil {
		defer key.Close()
		path, _, err := key.GetStringValue("BinaryPath")
		if err == nil && path != "" {
			logger.Logf("Found existing system installation from registry (BinaryPath): %s", path)
			return path, true
		}
		path, _, err = key.GetStringValue("InstallPath")
		if err == nil && path != "" {
			logger.Logf("Found existing system installation from registry (InstallPath): %s", path)
			return path, true
		}
	}

	logger.Log("No existing installation found in registry.")
	return "", false
}

func performInstall(logger *Logger, btn *widget.Button) {
	defer func() {
		btn.Enable()
	}()

	logger.Log("Starting Inker for Windows installation...")
	logger.Log("==================================================")

	tmpDir, err := os.MkdirTemp("", "inker-install-*")
	if err != nil {
		logger.Logf("ERROR: failed to create temp directory: %v", err)
		return
	}
	defer func() {
		logger.Logf("Cleaning up temp directory: %s", tmpDir)
		os.RemoveAll(tmpDir)
	}()
	logger.Logf("Created temp directory: %s", tmpDir)

	zipPath := filepath.Join(tmpDir, "inker.zip")
	logger.Logf("Downloading from: %s", inkerURL)
	if err := downloadFile(inkerURL, zipPath, logger); err != nil {
		logger.Logf("ERROR: download failed: %v", err)
		return
	}
	logger.Logf("Successfully downloaded: %s", zipPath)

	extractDir := filepath.Join(tmpDir, "inker")
	logger.Logf("Extracting to: %s", extractDir)
	if err := extractZip(zipPath, extractDir, logger); err != nil {
		logger.Logf("ERROR: extraction failed: %v", err)
		return
	}
	logger.Log("Successfully extracted inker.zip")

	var inkerExePath string
	err = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".exe") && strings.Contains(strings.ToLower(info.Name()), "inker") {
			inkerExePath = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil || inkerExePath == "" {
		logger.Log("ERROR: inker.exe not found in extracted files")
		return
	}
	logger.Logf("Found executable: %s", inkerExePath)

	var destPath string
	if path, found := findInkerInstallPath(logger); found {
		destPath = path
	} else {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			logger.Log("ERROR: USERPROFILE environment variable not set")
			return
		}
		userBin := filepath.Join(userProfile, "bin")
		logger.Logf("Defaulting to new user installation in: %s", userBin)
		destPath = filepath.Join(userBin, "inker.exe")
	}

	logger.Logf("Target installation path: %s", destPath)

	// On Windows, a running executable cannot be overwritten.
	// The common pattern is to rename the old one and then copy the new one.
	if _, err := os.Stat(destPath); err == nil {
		logger.Log("Existing inker.exe found. Attempting to move it before update.")
		oldPath := destPath + ".old"

		// If an even older .old file exists, try to remove it.
		if _, err := os.Stat(oldPath); err == nil {
			logger.Logf("Removing previous backup file: %s", oldPath)
			if err := os.Remove(oldPath); err != nil {
				logger.Logf("WARNING: could not remove old backup file: %v", err)
			}
		}

		// Rename current executable to .old
		if err := os.Rename(destPath, oldPath); err != nil {
			logger.Logf("ERROR: failed to move existing executable: %v", err)
			logger.Log("This may be because Inker is currently running or due to permissions. Please close Inker and try again. If it still fails, try running the installer as an administrator.")
			return
		}
		logger.Logf("Moved existing executable to: %s", oldPath)
	}

	// Ensure the destination directory exists, especially for registry-found paths
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logger.Logf("ERROR: failed to create destination directory %s: %v", destDir, err)
		return
	}

	logger.Logf("Copying new inker.exe to: %s", destPath)
	if err := copyFile(inkerExePath, destPath); err != nil {
		logger.Logf("ERROR: failed to copy executable: %v", err)
		logger.Log("This could be a permissions issue. Try running the installer as an administrator.")
		// Try to restore the old executable if we moved it
		oldPath := destPath + ".old"
		if _, statErr := os.Stat(oldPath); statErr == nil {
			if renameErr := os.Rename(oldPath, destPath); renameErr == nil {
				logger.Log("Restored previous version of inker.exe")
			}
		}
		return
	}
	logger.Log("Successfully installed inker.exe")

	appData := os.Getenv("APPDATA")
	if appData != "" {
		startMenu := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
		logger.Logf("Creating Start Menu entry: %s", startMenu)
		if err := os.MkdirAll(startMenu, 0755); err == nil {
			cmdFile := filepath.Join(startMenu, "Inker.cmd")
			cmdContent := fmt.Sprintf("@echo off\n\"%s\" %%*\n", destPath)
			if err := os.WriteFile(cmdFile, []byte(cmdContent), 0644); err == nil {
				logger.Logf("Created Start Menu shortcut: %s", cmdFile)
			} else {
				logger.Logf("WARNING: failed to create Start Menu entry: %v", err)
			}
		}
	}

	logger.Log("==================================================")
	logger.Log("Initialising FtR registry...")

	logger.Log("Writing to Windows registry...")
	keyParent := `Software\FtR\Packages`
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyParent, registry.ALL_ACCESS)
	if err != nil {
		logger.Logf("WARNING: failed to access registry parent key: %v", err)
	} else {
		defer k.Close()
		appKey, _, err := registry.CreateKey(k, "inker", registry.ALL_ACCESS)
		if err != nil {
			logger.Logf("WARNING: failed to create inker registry key: %v", err)
		} else {
			defer appKey.Close()
			if err := appKey.SetStringValue("Version", "2.7.0"); err == nil {
				logger.Log("Wrote Version=2.7.0 to registry")
			}
			if err := appKey.SetStringValue("InstallPath", destPath); err == nil {
				logger.Log("Wrote InstallPath to registry")
			}
			if err := appKey.SetStringValue("BinaryPath", destPath); err == nil {
				logger.Log("Wrote BinaryPath to registry")
			}
		}
	}

	logger.Log("Updating FtR JSON registry...")
	localApp := os.Getenv("LOCALAPPDATA")
	if localApp == "" {
		logger.Log("WARNING: LOCALAPPDATA not set; skipping JSON registry")
	} else {
		regDir := filepath.Join(localApp, "ftr")
		regFile := filepath.Join(regDir, "registry.json")
		logger.Logf("JSON registry path: %s", regFile)
		if err := os.MkdirAll(regDir, 0755); err != nil {
			logger.Logf("WARNING: failed to create registry directory: %v", err)
		} else {
			type PackageInfo struct {
				Name        string    `json:"name"`
				Version     string    `json:"version,omitempty"`
				Source      string    `json:"source,omitempty"` // user/repo
				BinaryPath  string    `json:"binary_path,omitempty"`
				InstalledAt time.Time `json:"installed_at"`
			}
			// This struct should mirror the one in inker4win/pkg/registry to ensure compatibility.
			type RegistryFile struct {
				Packages        []PackageInfo `json:"packages"`
				LastSync        time.Time     `json:"last_sync"`
				SyncError       string        `json:"sync_error,omitempty"`
				InkerVersion    string        `json:"inker_version"`
				RegistryVersion string        `json:"registry_version"`
			}

			var data RegistryFile
			if b, err := os.ReadFile(regFile); err == nil {
				_ = json.Unmarshal(b, &data)
				logger.Log("Loaded existing registry file")
			} else {
				logger.Log("Creating new registry file")
				// Initialize with default values if creating a new one
				data.RegistryVersion = "2.0"
			}

			entry := PackageInfo{
				Name:        "inker",
				Version:     "2.7.0",
				Source:      "qchef/inker",
				BinaryPath:  destPath,
				InstalledAt: time.Now(),
			}

			replaced := false
			for i := range data.Packages {
				if data.Packages[i].Name == entry.Name {
					data.Packages[i] = entry
					replaced = true
					break
				}
			}
			if !replaced {
				data.Packages = append(data.Packages, entry)
			}

			if out, err := json.MarshalIndent(data, "", "  "); err == nil {
				if err := os.WriteFile(regFile, out, 0644); err == nil {
					logger.Log("Successfully wrote package info to JSON registry")
				} else {
					logger.Logf("WARNING: failed to write registry file: %v", err)
				}
			}
		}
	}

	logger.Log("==================================================")
	logger.Log("Installation completed successfully!")
	logger.Logf("Inker is now available at: %s", destPath)
	logger.Log("You can run it from the Start Menu or by typing 'inker' in a terminal.")
}

func downloadFile(url, destPath string, logger *Logger) error {
	logger.Logf("Starting download: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	logger.Logf("Response status: %d", resp.StatusCode)
	contentLength := resp.ContentLength
	if contentLength > 0 {
		logger.Logf("Content length: %.2f MB", float64(contentLength)/1024/1024)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	logger.Logf("Downloaded %.2f MB", float64(written)/1024/1024)
	return nil
}

func extractZip(zipPath, destDir string, logger *Logger) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	logger.Logf("Zip contains %d files", len(reader.File))

	for i, f := range reader.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
		} else {
			if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			inFile, err := f.Open()
			if err != nil {
				outFile.Close()
				return err
			}
			_, err = io.Copy(outFile, inFile)
			inFile.Close()
			outFile.Close()
			if err != nil {
				return err
			}
			if i < 10 || i%10 == 0 {
				logger.Logf("  Extracted: %s (%d bytes)", f.Name, f.UncompressedSize64)
			}
		}
	}
	return nil
}

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
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}
