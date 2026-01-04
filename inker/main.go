package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/color"
	"inker/api"
	"inker/builder"
	"inker/fsdl"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	appName    = "FtR Inker 2.5"
	appWidth   = 800
	appHeight  = 600
	guiWorkers = 4
)

type AppSettings struct {
	Theme          string `json:"theme"`
	DownloadPath   string `json:"download_path"`
	DownloadAsk    bool   `json:"download_ask"`
	SyncMode       string `json:"sync_mode"` // "Default", "Custom", "Ask"
	SyncCustomPath string `json:"sync_custom_path"`
	downloadPathM  string
	// Auto sync settings
	AutoSyncIntervalMinutes int             `json:"auto_sync_interval_minutes"`
	AutoSyncEntries         []AutoSyncEntry `json:"auto_sync_entries"`
	AutoSyncIntervalSeconds int             `json:"auto_sync_interval_seconds"`
}

type LocalFileInfo struct {
	Info os.FileInfo
	Hash string
}

type AutoSyncEntry struct {
	User           string `json:"user"`
	Repo           string `json:"repo"`
	SyncMode       string `json:"sync_mode"`
	SyncCustomPath string `json:"sync_custom_path"`
	ShowReceipt    bool   `json:"show_receipt"`
}

var (
	appSettings AppSettings
	ftrClient   *api.Client
	uiQueue     chan func()
	w           fyne.Window
)

var updateUI func()

func main() {
	// Channel to queue UI updates from brackground goroutines
	uiQueue = make(chan func(), 100)
	resetCountdown := make(chan int, 1)

	a := app.NewWithID("0")
	loadSettings(a)

	// Destination directory
	var downDest string

	drv, ok := a.Driver().(desktop.Driver)

	if ok && runtime.GOOS != "darwin" {
		w = drv.CreateSplashWindow()
	} else {
		w = a.NewWindow(appName)
	}

	// w = a.NewWindow(appName)

	var err error
	ftrClient, err = api.NewClient()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to establish connection with InkDrop server: %w", err), w)
	}

	var searchResults []map[string]string
	resultsList := widget.NewList(
		func() int {
			return len(searchResults)
		},
		func() fyne.CanvasObject {
			buttonBox := container.NewHBox(
				widget.NewButtonWithIcon("Install", theme.DownloadIcon(), nil),
				widget.NewButtonWithIcon("Download", theme.DownloadIcon(), nil),
				widget.NewButtonWithIcon("Sync", theme.ViewRefreshIcon(), nil),
				widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), nil),
			)
			labelBox := container.NewVBox(
				widget.NewLabel("user/repo"),
				widget.NewLabel("description"),
			)
			return container.NewBorder(
				nil, nil, nil, buttonBox, labelBox,
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i >= len(searchResults) {
				return
			}
			item := searchResults[i]
			user, repo := item["user"], item["repo"]
			repoPath := fmt.Sprintf("%s/%s", user, repo)
			desc := item["description"]
			if desc == "" {
				desc = "(no description)"
			}

			borderContainer := o.(*fyne.Container)
			for _, obj := range borderContainer.Objects {
				switch v := obj.(type) {
				case *fyne.Container:
					if len(v.Objects) >= 2 { // Can be 2 (labels) or 3 (buttons)
						if _, ok := v.Objects[0].(*widget.Button); ok {
							getBtn := v.Objects[0].(*widget.Button)
							downBtn := v.Objects[1].(*widget.Button)
							syncBtn := v.Objects[2].(*widget.Button)
							addBtn := v.Objects[3].(*widget.Button)

							// Get button event handling - download .fsdl file in a repository and install it to the user's system
							getBtn.OnTapped = func() {
								info := fmt.Sprintf("User: %s, Repository: %s", user, repo)
								log.Printf("Install button clicked for: %s", info)

								go func(u, r string) {
									statusLabel := widget.NewLabel("Preparing to install...")
									fileProgressLabel := widget.NewLabel("")
									overallProgress := widget.NewProgressBar()
									fileProgress := widget.NewProgressBar()

									progressDialog := dialog.NewCustomWithoutButtons("Installing...", container.NewVBox(statusLabel, overallProgress, fileProgressLabel, fileProgress), w)
									uiQueue <- func() { progressDialog.Show() }
									defer func() { uiQueue <- func() { progressDialog.Hide() } }()

									repoPath := fmt.Sprintf("%s/%s", user, repo)
									tmpDir := "/tmp/fsdl"
									if err := os.MkdirAll(tmpDir, 0755); err != nil {
										log.Printf("failed to create temp directory: %v", err)
										uiQueue <- func() {
											progressDialog.Hide()
											dialog.ShowError(fmt.Errorf("failed to create temp directory: %w", err), w)
										}
										return
									}

									fsdlFile := filepath.Join(tmpDir, repo+".fsdl")

									// Download from server
									log.Printf("Fetching repo %s", repoPath)
									uiQueue <- func() { statusLabel.SetText(fmt.Sprintf("Downloading metadata for %s", repoPath)) }

									log.Println("Fetching package via API...")
									// Use repo.php API to download and verify
									if err := ftrClient.DownloadAndVerify(user, repo, repo+".fsdl", fsdlFile, func(p float64) {
										uiQueue <- func() {
											if overallProgress.Value < 0.3 {
												overallProgress.SetValue(0.1 + p*0.2) // Download is 10-30% of overall
											}
											fileProgress.SetValue(p)
										}
									}); err != nil { // Check for the specific "file not found" error for the FSDL file.
										if err.Error() == fmt.Sprintf("file not found: %s", repo+".fsdl") {
											uiQueue <- func() {
												dialog.ShowInformation("Not Found", fmt.Sprintf("The required installer file (%s.fsdl) was not found in this repository.", repo), w)
											}
										} else {
											log.Printf("download failed: %v", err)
											uiQueue <- func() {
												progressDialog.Hide()
												dialog.ShowError(fmt.Errorf("metadata download failed: %w", err), w)
											}
										}
										return
									}

									uiQueue <- func() {
										statusLabel.SetText("Extracting package...")
										overallProgress.SetValue(0.3)
									}
									if err := fsdl.Extract(fsdlFile, tmpDir); err != nil {
										log.Printf("failed to extract package: %v", err)
										uiQueue <- func() {
											progressDialog.Hide()
											dialog.ShowError(fmt.Errorf("failed to extract package: %w", err), w)
										}
										return
									}

									b := builder.New(repo, tmpDir)

									binaryPath, err := b.DetectAndBuild()
									if err != nil {
										log.Printf("build failed: %v", err)
										uiQueue <- func() {
											progressDialog.Hide()
											dialog.ShowError(fmt.Errorf("build failed: %w", err), w)
										}
										return
									}

									if binaryPath != "" {
										// Prompt for sudo password
										pwdChan := make(chan string)
										cancelChan := make(chan struct{})
										uiQueue <- func() {
											pwdEntry := widget.NewPasswordEntry()
											dialog.ShowCustomConfirm("Authentication Required", "Install", "Cancel",
												container.NewVBox(
													widget.NewLabel("Enter your sudo password to install this package:"),
													pwdEntry,
												),
												func(ok bool) {
													if ok {
														pwdChan <- pwdEntry.Text
													} else {
														close(cancelChan)
													}
												}, w)
										}

										select {
										case password := <-pwdChan:
											b.SetPassword(password)
										case <-cancelChan:
											uiQueue <- func() {
												progressDialog.Hide()
												dialog.ShowInformation("Cancelled", "Installation cancelled.", w)
											}
											return
										}

										uiQueue <- func() {
											statusLabel.SetText("Installing binary (requires privileges)...")
											overallProgress.SetValue(0.8)
										}
										if err := b.InstallBinary(binaryPath); err != nil {
											log.Printf("installation failed: %v", err)
											uiQueue <- func() {
												progressDialog.Hide()
												dialog.ShowError(fmt.Errorf("installation failed: %w", err), w)
											}
											return
										}
									}

									uiQueue <- func() {
										done := make(chan struct{})
										overallProgress.SetValue(1.0)
										progressDialog.Hide()
										infoDialog := dialog.NewInformation("Install complete", fmt.Sprintf("Finished installing %s/%s", user, repo), w)
										infoDialog.SetOnClosed(func() { close(done) })
										infoDialog.Show()
									}
								}(user, repo)
							}

							downBtn.OnTapped = func() {
								info := fmt.Sprintf("User: %s, Repository: %s", user, repo)
								log.Printf("Download button clicked for: %s", info)

								go func(user, repo string) {
									if ftrClient == nil {
										log.Println("Download failed: client is not initialised.")
										uiQueue <- func() {
											dialog.ShowError(fmt.Errorf("client is not initialised, cannot download files"), w)
										}
										return
									}

									statusLabel := widget.NewLabel("Downloading...")
									overallProgress := widget.NewProgressBar()

									workerContainer := container.NewVBox()
									workerBars := make([]*widget.ProgressBar, guiWorkers)
									workerLabels := make([]*widget.Label, guiWorkers)
									for i := 0; i < guiWorkers; i++ {
										workerLabels[i] = widget.NewLabel(fmt.Sprintf("Worker %d: Idle", i+1))
										workerBars[i] = widget.NewProgressBar()
										workerContainer.Add(workerLabels[i])
										workerContainer.Add(workerBars[i])
									}

									progressDialog := dialog.NewCustomWithoutButtons("Downloading...", container.NewVBox(statusLabel, overallProgress, widget.NewSeparator(), workerContainer), w)

									uiQueue <- func() { progressDialog.Show() }

									log.Printf("Listing files in %s/%s...", user, repo)
									files, err := ftrClient.ListRepoFiles(user, repo)
									if err != nil {
										log.Printf("failed to list repository files: %v", err)
										uiQueue <- func() {
											dialog.ShowError(fmt.Errorf("failed to list repository files: %w", err), w)
											progressDialog.Hide()
										}
										return
									}
									defer func() { uiQueue <- func() { progressDialog.Hide() } }() // Defer hide after we know there's no early return

									if len(files) == 0 {
										log.Println("No files were found in the repository.")
										progressDialog.Hide()
										uiQueue <- func() {
											dialog.ShowInformation("Empty Repository", "No files were found in the repository.", w)
										}
										return
									}

									var totalSize int64
									for _, f := range files {
										if size, ok := f["size"].(float64); ok {
											totalSize += int64(size)
										}
									}

									home, _ := os.UserHomeDir()
									dest := filepath.Join(home, "FtRSync", user, repo)
									if downDest != "" {
										dest = downDest
									}

									if err := os.MkdirAll(dest, 0755); err != nil {
										log.Printf("failed to create destination directory: %v", err)
										uiQueue <- func() {
											progressDialog.Hide()
											dialog.ShowError(fmt.Errorf("failed to create destination directory: %w", err), w)
										}
										return
									}

									errorsList := []string{}
									var downloadedSize int64

									type downloadTask struct {
										path string
										size int64
									}
									taskChan := make(chan downloadTask, len(files))
									for _, f := range files {
										if p, ok := f["path"].(string); ok && p != "" {
											s, _ := f["size"].(float64)
											taskChan <- downloadTask{path: p, size: int64(s)}
										}
									}
									close(taskChan)

									var wg sync.WaitGroup
									for i := 0; i < guiWorkers; i++ {
										wg.Add(1)
										go func(workerID int) {
											defer wg.Done()
											for task := range taskChan {
												uiQueue <- func() {
													workerLabels[workerID].SetText(fmt.Sprintf("Downloading: %s", task.path))
													workerBars[workerID].SetValue(0)
												}

												var lastP float64

												fullPath := filepath.Join(dest, filepath.FromSlash(task.path))
												if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
													// Log error safely
													continue
												}

												err := ftrClient.DownloadAndVerify(user, repo, task.path, fullPath, func(p float64) {
													deltaP := p - lastP
													lastP = p
													deltaBytes := int64(deltaP * float64(task.size))
													newTotal := atomic.AddInt64(&downloadedSize, deltaBytes)

													uiQueue <- func() {
														workerBars[workerID].SetValue(p)
														if totalSize > 0 {
															overallProgress.SetValue(float64(newTotal) / float64(totalSize))
														}
													}
												})
												if err != nil {
													// Collect errors safely
												}

												uiQueue <- func() {
													workerLabels[workerID].SetText(fmt.Sprintf("Worker %d: Idle", workerID+1))
													workerBars[workerID].SetValue(0)
												}
											}
										}(i)
									}
									wg.Wait()

									done := make(chan struct{})
									uiQueue <- func() {
										if len(errorsList) > 0 {
											progressDialog.Hide()
											log.Printf("Errors encountered during download for %s/%s:\n%v", user, repo, errorsList)
											errorDialog := dialog.NewInformation("Encountered errors", fmt.Sprintf("Encountered errors trying to download %s/%s:\n%v", user, repo, errorsList), w)
											errorDialog.SetOnClosed(func() { close(done) })
											errorDialog.Show()
										} else {
											progressDialog.Hide()
											successDialog := dialog.NewInformation("Download Complete", fmt.Sprintf("Finished downloading %s/%s", user, repo), w)
											successDialog.SetOnClosed(func() { close(done) })
											successDialog.Show()
										}
										log.Println("All files processed.")
									}
									<-done
								}(user, repo)
								log.Printf("Download process for %s/%s initiated.", user, repo)
							}

							syncBtn.OnTapped = func() {
								log.Printf("Sync button tapped for %s/%s", user, repo)
								go func(user, repo string) {
									var syncDir string
									var err error

									askDone := make(chan struct{})

									uiQueue <- func() {
										switch appSettings.SyncMode {
										case "Ask":
											dialog.ShowFolderOpen(func(uri fyne.ListableURI, errDialog error) {
												if errDialog != nil {
													err = fmt.Errorf("dialog error: %w", errDialog)
												} else if uri == nil {
													err = fmt.Errorf("sync cancelled by user")
												} else {
													syncDir = uri.Path()
												}
												close(askDone)
											}, w)
										case "Custom":
											syncDir = appSettings.SyncCustomPath
											if syncDir == "" {
												err = fmt.Errorf("custom sync path is not set. Please set it in Settings")
											}
											close(askDone)
										default: // "Default"
											home, homeErr := os.UserHomeDir()
											if homeErr != nil {
												err = fmt.Errorf("failed to determine home directory: %w", homeErr)
											} else {
												syncDir = filepath.Join(home, "FtRSync", user, repo)
											}
											close(askDone)
										}
									}

									<-askDone

									if err != nil {
										log.Println(err)
										if err.Error() != "sync cancelled by user" {
											uiQueue <- func() { dialog.ShowError(err, w) }
										}
										return
									}
									if syncDir == "" {
										log.Println("Sync directory not specified.")
										return
									}

									// --- Start Sync Logic ---
									statusLabel := widget.NewLabel("Starting sync...")
									overallProgress := widget.NewProgressBar()

									workerContainer := container.NewVBox()
									workerBars := make([]*widget.ProgressBar, guiWorkers)
									workerLabels := make([]*widget.Label, guiWorkers)
									for i := 0; i < guiWorkers; i++ {
										workerLabels[i] = widget.NewLabel(fmt.Sprintf("Worker %d: Idle", i+1))
										workerBars[i] = widget.NewProgressBar()
										workerContainer.Add(workerLabels[i])
										workerContainer.Add(workerBars[i])
									}

									progressDialog := dialog.NewCustomWithoutButtons("Synchronising...", container.NewVBox(statusLabel, overallProgress, widget.NewSeparator(), workerContainer), w)
									uiQueue <- func() { progressDialog.Show() }
									defer func() { uiQueue <- func() { progressDialog.Hide() } }()

									uiQueue <- func() { statusLabel.SetText(fmt.Sprintf("Syncing with %s", syncDir)) }
									if err := os.MkdirAll(syncDir, 0755); err != nil {
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to create sync directory: %w", err), w) }
										return
									}

									// 1. List remote files
									uiQueue <- func() { statusLabel.SetText("Listing remote files...") }
									remoteFiles, err := ftrClient.ListRepoFiles(user, repo)
									if err != nil {
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to list remote files: %w", err), w) }
										return
									}

									// 2. List local files
									uiQueue <- func() { statusLabel.SetText("Scanning local files...") }
									localFiles := make(map[string]LocalFileInfo)
									err = filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
										if err != nil {
											return err
										}
										if !info.IsDir() {
											rel, _ := filepath.Rel(syncDir, path)
											rel = filepath.ToSlash(rel)

											f, err := os.Open(path)
											if err != nil {
												return err
											}
											defer f.Close()

											h := sha256.New()
											if _, err := io.Copy(h, f); err != nil {
												return err
											}
											localFiles[rel] = LocalFileInfo{Info: info, Hash: fmt.Sprintf("%x", h.Sum(nil))}
										}
										return nil
									})
									if err != nil {
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to scan and hash local files: %w", err), w) }
										return
									}

									// 3. Compare and build task lists (basic implementation)
									uiQueue <- func() { statusLabel.SetText("Comparing files...") }
									uploads := []string{}
									downloads := []string{}
									conflicts := []string{}

									remoteMap := make(map[string]map[string]interface{})
									for _, rf := range remoteFiles {
										if path, ok := rf["path"].(string); ok {
											remoteMap[path] = rf
										}
									}

									for localPath, localFile := range localFiles {
										remoteFile, exists := remoteMap[localPath]
										if !exists {
											uploads = append(uploads, localPath)
										} else {
											if remoteHash, ok := remoteFile["hash"].(string); ok {
												if remoteHash != localFile.Hash {
													conflicts = append(conflicts, localPath)
												}
											}
										}
									}

									for remotePath := range remoteMap {
										if _, exists := localFiles[remotePath]; !exists {
											downloads = append(downloads, remotePath)
										}
									}

									uiQueue <- func() {
										statusLabel.SetText(fmt.Sprintf("Found %d files to upload and %d files to download.", len(uploads), len(downloads)))
									}
									// For now, we will treat conflicts as files to be downloaded, overwriting local changes.
									// A future update could add a conflict resolution dialog.
									downloads = append(downloads, conflicts...)

									var totalDownloadSize int64
									for _, path := range downloads {
										if remoteFile, ok := remoteMap[path]; ok {
											if size, ok := remoteFile["size"].(float64); ok {
												totalDownloadSize += int64(size)
											}
										}
									}

									var totalUploadSize int64
									for _, path := range uploads {
										if localFile, ok := localFiles[path]; ok {
											totalUploadSize += localFile.Info.Size()
										}
									}

									totalSyncSize := totalDownloadSize + totalUploadSize
									var syncedSize int64

									// 4. Execute tasks (parallel)
									type syncTask struct {
										op   string // "download" or "upload"
										path string
										size int64
									}
									taskChan := make(chan syncTask, len(downloads)+len(uploads))
									for _, path := range downloads {
										size := 0.0
										if rf, ok := remoteMap[path]; ok {
											size, _ = rf["size"].(float64)
										}
										taskChan <- syncTask{op: "download", path: path, size: int64(size)}
									}
									for _, path := range uploads {
										size := int64(0)
										if lf, ok := localFiles[path]; ok {
											size = lf.Info.Size()
										}
										taskChan <- syncTask{op: "upload", path: path, size: size}
									}
									close(taskChan)

									var wg sync.WaitGroup
									for i := 0; i < guiWorkers; i++ {
										wg.Add(1)
										go func(workerID int) {
											defer wg.Done()
											for task := range taskChan {
												uiQueue <- func() {
													workerLabels[workerID].SetText(fmt.Sprintf("%s: %s", strings.Title(task.op), task.path))
													workerBars[workerID].SetValue(0)
												}

												var lastP float64
												progressCb := func(p float64) {
													deltaP := p - lastP
													lastP = p
													deltaBytes := int64(deltaP * float64(task.size))
													newTotal := atomic.AddInt64(&syncedSize, deltaBytes)

													uiQueue <- func() {
														workerBars[workerID].SetValue(p)
														if totalSyncSize > 0 {
															overallProgress.SetValue(float64(newTotal) / float64(totalSyncSize))
														}
													}
												}

												if task.op == "download" {
													destPath := filepath.Join(syncDir, filepath.FromSlash(task.path))
													os.MkdirAll(filepath.Dir(destPath), 0755)
													ftrClient.DownloadAndVerify(user, repo, task.path, destPath, progressCb)
												} else {
													localPath := filepath.Join(syncDir, filepath.FromSlash(task.path))
													if file, err := os.Open(localPath); err == nil {
														ftrClient.UploadFile(repoPath, task.path, file, task.size, false, progressCb)
														file.Close()
													}
												}

												uiQueue <- func() {
													workerLabels[workerID].SetText(fmt.Sprintf("Worker %d: Idle", workerID+1))
													workerBars[workerID].SetValue(0)
												}
											}
										}(i)
									}
									wg.Wait()

									done := make(chan struct{})
									uiQueue <- func() {
										overallProgress.SetValue(1.0)
										progressDialog.Hide()
										infoDialog := dialog.NewInformation("Sync Complete", fmt.Sprintf("Finished syncing %s/%s.", user, repo), w)
										infoDialog.SetOnClosed(func() { close(done) })
										infoDialog.Show()
									}
									<-done

								}(user, repo)
							}

							addBtn.OnTapped = func() {
								log.Printf("Add to Auto Sync: %s/%s", user, repo)
								found := false
								for _, e := range appSettings.AutoSyncEntries {
									if e.User == user && e.Repo == repo {
										found = true
										break
									}
								}
								if !found {
									appSettings.AutoSyncEntries = append(appSettings.AutoSyncEntries, AutoSyncEntry{User: user, Repo: repo, SyncMode: appSettings.SyncMode, SyncCustomPath: appSettings.SyncCustomPath, ShowReceipt: false})
									saveSettings()
									dialog.ShowInformation("Added", fmt.Sprintf("Added %s/%s to Auto Sync", user, repo), w)
								} else {
									dialog.ShowInformation("Info", "Repository is already in Auto Sync list.", w)
								}
							}
						} else {
							v.Objects[0].(*widget.Label).SetText(repoPath)
							v.Objects[1].(*widget.Label).SetText(desc)
						}
					}
				}
			}
		},
	)

	// Placeholder content
	placeholder := widget.NewLabel("Use the search bar above to find repositories.")
	placeholder.Alignment = fyne.TextAlignCenter
	content := container.NewStack(placeholder, resultsList)

	// Search Logic
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search for repositories")

	searchEntry.OnSubmitted = func(query string) {
		if query != "" {
			go func() {
				log.Printf("Searching for %s", query)
				matches, err := ftrClient.SearchRepos(query)
				if err != nil {
					uiQueue <- func() { dialog.ShowError(err, w) }
					return
				}

				uiQueue <- func() {
					log.Printf("Search returned %d matches.", len(matches))
					searchResults = matches
					if len(searchResults) > 0 {
						placeholder.Hide()
					} else {
						placeholder.SetText("No matches found.")
						placeholder.Show()
					}
					resultsList.Refresh()
				}
			}()
		}
	}

	// Defining App Tabs

	// Search tab

	searchTabContent := container.NewBorder(
		container.NewVBox(searchEntry),
		nil, nil, nil,
		content,
	)

	// Login tab

	loginStatusLabel := widget.NewLabel("You are not logged in. Enter your InkDrop credentials to log in.")
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Email")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")
	loginButton := widget.NewButton("Login", nil)
	loginForm := container.NewVBox(
		loginStatusLabel,
		emailEntry,
		passwordEntry,
		loginButton,
	)

	// Account tab

	accountStatusLabel := widget.NewLabel("Not logged in.")
	logoutButton := widget.NewButton("Logout", nil)
	accountTabContent := container.NewVBox(
		accountStatusLabel,
		logoutButton,
	)

	// --- Settings Tab ---
	themeRadio := widget.NewRadioGroup([]string{"System", "Light", "Dark"}, func(s string) {
		appSettings.Theme = s
		applyTheme(a)
		saveSettings()
	})
	themeRadio.Horizontal = true

	downloadPathEntry := widget.NewEntry()
	downloadPathEntry.SetText(appSettings.downloadPathM)
	downloadPathEntry.OnChanged = func(s string) {
		appSettings.downloadPathM = s
		saveSettings()
	}

	downloadPathSelectBtn := widget.NewButton("Select Path", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			appSettings.downloadPathM = uri.Path()
			downloadPathEntry.SetText(appSettings.downloadPathM)
			saveSettings()
		}, w)
	})

	downloadAskCheck := widget.NewCheck("Always ask where to save", func(b bool) {
		appSettings.DownloadAsk = b
		if b {
			downloadPathEntry.Disable()
			downloadPathSelectBtn.Disable()
		} else {
			downloadPathEntry.Enable()
			downloadPathSelectBtn.Enable()
		}
		saveSettings()
	})
	downloadAskCheck.SetChecked(appSettings.DownloadAsk)

	settingsTabContent := container.NewVBox(
		widget.NewLabelWithStyle("Theme", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		themeRadio,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Browser Download Path", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		downloadAskCheck,
		container.NewBorder(nil, nil, nil, downloadPathSelectBtn, downloadPathEntry),
	)

	settingsTabContent.Add(widget.NewSeparator())
	settingsTabContent.Add(widget.NewLabelWithStyle("Sync Download Path", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

	syncPathEntry := widget.NewEntry()
	syncPathEntry.SetText(appSettings.SyncCustomPath)
	syncPathEntry.OnChanged = func(s string) {
		appSettings.SyncCustomPath = s
		saveSettings()
	}

	syncPathSelectBtn := widget.NewButton("Select Path", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			appSettings.SyncCustomPath = uri.Path()
			syncPathEntry.SetText(appSettings.SyncCustomPath)
			saveSettings()
		}, w)
	})

	syncModeRadio := widget.NewRadioGroup([]string{"Default (~/FtRSync/user/repo)", "Always Sync To...", "Ask Every Time"}, func(s string) {
		switch s {
		case "Always Sync To...":
			appSettings.SyncMode = "Custom"
			syncPathEntry.Enable()
			syncPathSelectBtn.Enable()
		case "Ask Every Time":
			appSettings.SyncMode = "Ask"
			syncPathEntry.Disable()
			syncPathSelectBtn.Disable()
		default:
			appSettings.SyncMode = "Default"
			syncPathEntry.Disable()
			syncPathSelectBtn.Disable()
		}
		saveSettings()
	})

	switch appSettings.SyncMode {
	case "Custom":
		syncModeRadio.SetSelected("Always Sync To...")
	case "Ask":
		syncModeRadio.SetSelected("Ask Every Time")
		syncPathEntry.Disable()
		syncPathSelectBtn.Disable()
	default:
		syncModeRadio.SetSelected("Default (~/FtRSync/user/repo)")
		syncPathEntry.Disable()
		syncPathSelectBtn.Disable()
	}

	settingsTabContent.Add(syncModeRadio)
	settingsTabContent.Add(container.NewBorder(nil, nil, nil, syncPathSelectBtn, syncPathEntry))

	// Auto Sync Interval UI (debug-aware)
	debugMode := os.Getenv("FTR_DEBUG") != ""
	var sliderMax float64
	var sliderStep float64
	if debugMode {
		// debug: 0..600 seconds, 20s min typical; 0 means disabled
		sliderMax = 600
		sliderStep = 5
	} else {
		// production: 0..7200 seconds (0..2h), step 600s (10min)
		sliderMax = 7200
		sliderStep = 600
	}
	settingsTabContent.Add(widget.NewLabelWithStyle("Auto Sync Interval:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	intervalSlider := widget.NewSlider(0, sliderMax)
	intervalSlider.Step = sliderStep
	// initialize from seconds (backwards compatibility)
	if appSettings.AutoSyncIntervalSeconds > 0 {
		intervalSlider.SetValue(float64(appSettings.AutoSyncIntervalSeconds))
	} else {
		intervalSlider.SetValue(0)
	}

	valueLabel := widget.NewLabel("")
	updateValueLabel := func(v float64) {
		if int(v) == 0 {
			valueLabel.SetText("Disabled")
			return
		}
		if debugMode {
			valueLabel.SetText(fmt.Sprintf("%d seconds", int(v)))
		} else {
			mins := int(v) / 60
			valueLabel.SetText(fmt.Sprintf("%d minutes", mins))
		}
	}
	updateValueLabel(intervalSlider.Value)

	intervalSlider.OnChanged = func(v float64) {
		appSettings.AutoSyncIntervalSeconds = int(v)
		if !debugMode {
			appSettings.AutoSyncIntervalMinutes = int(v) / 60
		} else {
			appSettings.AutoSyncIntervalMinutes = int(v) / 60
		}
		saveSettings()
		updateValueLabel(v)
		log.Printf("AutoSync interval set to %d seconds", appSettings.AutoSyncIntervalSeconds)
		if appSettings.AutoSyncIntervalSeconds > 0 {
			select {
			case resetCountdown <- appSettings.AutoSyncIntervalSeconds:
			default:
				select {
				case <-resetCountdown:
				default:
				}
				resetCountdown <- appSettings.AutoSyncIntervalSeconds
			}
		}
	}

	settingsTabContent.Add(intervalSlider)
	settingsTabContent.Add(valueLabel)

	// visible countdown label
	countdownLabel := widget.NewLabel("Next sync: --:--")
	settingsTabContent.Add(countdownLabel)

	// --- Upload Tab ---
	var userRepos []map[string]string
	var selectedRepo string
	var selectedRepoUser string
	var selectedRepoID widget.ListItemID = -1
	var selectedFile []fyne.URI
	var encryptUpload bool
	var repoFiles []map[string]interface{}

	repoListLabel := widget.NewLabel("Your Repositories")
	selectedRepoLabel := widget.NewLabel("No repository selected")
	selectedFileLabel := widget.NewLabel("No files selected")
	uploadButton := widget.NewButton("Upload", nil)
	uploadButton.Disable()

	var repoList *widget.List

	updateUploadButtonState := func() {
		if selectedRepo != "" && len(selectedFile) > 0 {
			uploadButton.Enable()
		} else {
			uploadButton.Disable()
		}
	}

	onDeleteComplete := func() {
		repoList.OnSelected(selectedRepoID)
	}

	// File list for the selected repository
	fileList := widget.NewList(
		func() int {
			return len(repoFiles)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				container.NewHBox(
					widget.NewLabel("file.name"),
					widget.NewLabel("[ENCRYPTED]"),
				),
				widget.NewButton("View Hash", nil),
				widget.NewButtonWithIcon("Download", theme.DownloadIcon(), nil),
				widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(repoFiles) {
				return
			}
			file := repoFiles[id]
			fileName := file["path"].(string)
			isEncrypted := file["encrypted"].(bool)
			hash := file["hash"].(string)

			box := obj.(*fyne.Container)
			infoBox := box.Objects[0].(*fyne.Container)

			infoBox.Objects[0].(*widget.Label).SetText(fileName)
			encryptedLabel := infoBox.Objects[1].(*widget.Label)
			if isEncrypted {
				encryptedLabel.Show()
			} else {
				encryptedLabel.Hide()
			}

			viewHashBtn := box.Objects[1].(*widget.Button)
			viewHashBtn.OnTapped = func() {
				hashEntry := widget.NewMultiLineEntry()
				hashEntry.SetText(hash)
				hashEntry.Wrapping = fyne.TextWrapWord
				hashEntry.Disable()

				dialog.ShowCustomConfirm(
					"File Hash",
					"Copy",
					"Close",
					container.NewVBox(
						widget.NewLabel(fmt.Sprintf("SHA256 Hash for: %s", fileName)),
						hashEntry,
					),
					func(confirm bool) {
						if confirm {
							a.Clipboard().SetContent(hash)
						}
					}, w)
			}

			downloadBtnn := box.Objects[2].(*widget.Button)
			downloadBtnn.OnTapped = func() {
				log.Println("Download button clicked.")
				if appSettings.DownloadAsk {
					dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
						if err != nil {
							dialog.ShowError(err, w)
							return
						}
						if writer == nil {
							return
						}
						defer writer.Close()
						startDownload(selectedRepoUser, selectedRepo, fileName, writer.URI().Path())
					}, w)
				} else {
					destDir := appSettings.downloadPathM
					if destDir == "" {
						home, _ := os.UserHomeDir()
						destDir = filepath.Join(home, "Downloads")
					}
					destPath := filepath.Join(destDir, fileName)
					startDownload(selectedRepoUser, selectedRepo, fileName, destPath)
				}
			}

			deleteBtn := box.Objects[3].(*widget.Button)
			deleteBtn.OnTapped = func() {
				showDeleteConfirm(fileName, selectedRepoUser, selectedRepo, w, ftrClient, uiQueue, onDeleteComplete)
			}
		},
	)

	repoList = widget.NewList(
		func() int { return len(userRepos) },
		func() fyne.CanvasObject {
			return widget.NewLabel("user/repo")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(userRepos) {
				repo := userRepos[id]
				obj.(*widget.Label).SetText(fmt.Sprintf("%s/%s", repo["user"], repo["repo"]))
			}
		},
	)

	repoList.OnSelected = func(id widget.ListItemID) {
		if id < len(userRepos) {
			selectedRepoID = id
			repo := userRepos[id]
			selectedRepoUser = repo["user"]
			selectedRepoName := repo["repo"]
			selectedRepo = selectedRepoName
			selectedRepoLabel.SetText(fmt.Sprintf("Selected Repo: %s", selectedRepoName))
			updateUploadButtonState()

			go func(user, repoName string) {
				files, err := ftrClient.ListRepoFiles(user, repoName)
				if err != nil {
					uiQueue <- func() {
						dialog.ShowError(fmt.Errorf("failed to list files: %w", err), w)
					}
					return
				}
				uiQueue <- func() {
					repoFiles = files
					fileList.Refresh()
				}
			}(selectedRepoUser, selectedRepoName)
		}
	}

	selectFilesButton := widget.NewButton("Select File...", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			selectedFile = []fyne.URI{reader.URI()}
			selectedFileLabel.SetText(fmt.Sprintf("Selected File: %s", filepath.Base(reader.URI().Path())))
			updateUploadButtonState()
		}, w)
		fileDialog.Show()
	})

	encryptCheck := widget.NewCheck("Encrypt file on upload", func(checked bool) {
		encryptUpload = checked
	})
	encryptCheck.SetChecked(false)

	uploadButton.OnTapped = func() {
		go func() {
			repoToUpload := fmt.Sprintf("%s/%s", selectedRepoUser, selectedRepo)
			fileToUpload := selectedFile[0]
			fileName := filepath.Base(fileToUpload.Path())
			filePath := fileToUpload.Path()

			log.Printf("--- Upload Triggered ---")
			if !ftrClient.IsLoggedIn() {
				uiQueue <- func() {
					dialog.ShowInformation("Session Expired", "Your session has expired. Please log in again.", w)
					// Trigger logout to clear state
					if err := ftrClient.Logout(); err != nil {
						log.Printf("Error during automatic logout: %v", err)
					}
					updateUI()
				}
				return
			}

			fileInfo, err := os.Stat(filePath)
			if err != nil {
				uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to get file info: %w", err), w) }
				return
			}
			fileSize := fileInfo.Size()

			fileReader, err := os.Open(filePath)
			if err != nil {
				uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to open file: %w", err), w) }
				return
			}
			defer fileReader.Close()

			uploadProgress := widget.NewProgressBar()
			progressDialog := dialog.NewCustomWithoutButtons("Uploading...", container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Uploading %s...", fileName)),
				uploadProgress,
			), w)

			uiQueue <- func() { progressDialog.Show() }

			uploadErr := ftrClient.UploadFile(repoToUpload, fileName, fileReader, fileSize, encryptUpload, func(progress float64) {
				uiQueue <- func() { uploadProgress.SetValue(progress) }
			})

			done := make(chan struct{})
			uiQueue <- func() {
				progressDialog.Hide()
				if uploadErr != nil {
					log.Printf("Upload failed: %v", uploadErr)
					dialog.ShowError(fmt.Errorf("upload failed: %w", uploadErr), w)
				} else {
					dialog.ShowInformation("Success", fmt.Sprintf("Successfully uploaded %s to %s", fileName, selectedRepo), w)
					if repoList != nil {
						repoList.OnSelected(selectedRepoID)
					}
				}
				close(done)
			}
			<-done
		}()
	}

	browserRightPane := container.NewBorder(
		container.NewVBox(
			selectedRepoLabel,
			selectedFileLabel,
			selectFilesButton,
			encryptCheck,
			uploadButton,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		fileList,
	)

	// Rendering App tabs

	// --- Auto Sync Tab ---
	autoSyncLabel := widget.NewLabelWithStyle("Auto Sync", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var autoSyncSelected widget.ListItemID = -1
	var autoSyncList *widget.List
	autoSyncList = widget.NewList(
		func() int { return len(appSettings.AutoSyncEntries) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("user/repo (mode)")
			check := widget.NewCheck("Receipt", nil)
			syncBtn := widget.NewButtonWithIcon("Sync", theme.ViewRefreshIcon(), nil)
			prefsBtn := widget.NewButton("Sync Preferences", nil)
			return container.NewHBox(lbl, layout.NewSpacer(), check, syncBtn, prefsBtn)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			idx := int(i)
			if idx < len(appSettings.AutoSyncEntries) {
				e := appSettings.AutoSyncEntries[idx]
				box := o.(*fyne.Container)
				lbl := box.Objects[0].(*widget.Label)
				lbl.SetText(fmt.Sprintf("%s/%s (%s)", e.User, e.Repo, e.SyncMode))

				check := box.Objects[2].(*widget.Check)
				check.OnChanged = nil // Avoid triggering during binding
				check.SetChecked(e.ShowReceipt)
				check.OnChanged = func(b bool) {
					appSettings.AutoSyncEntries[idx].ShowReceipt = b
					saveSettings()
				}

				u := e.User
				r := e.Repo
				syncBtn := box.Objects[3].(*widget.Button)
				prefsBtn := box.Objects[4].(*widget.Button)

				syncBtn.OnTapped = func() {
					go performSync(u, r, appSettings.AutoSyncEntries[idx].SyncMode, appSettings.AutoSyncEntries[idx].SyncCustomPath, appSettings.AutoSyncEntries[idx].ShowReceipt)
				}
				prefsBtn.OnTapped = func() {
					// Preferences dialog for this auto-sync entry
					modeOptions := []string{"Default (~/FtRSync/user/repo)", "Always Sync To...", "Ask Every Time"}
					rg := widget.NewRadioGroup(modeOptions, func(s string) {})
					switch e.SyncMode {
					case "Custom":
						rg.SetSelected("Always Sync To...")
					case "Ask":
						rg.SetSelected("Ask Every Time")
					default:
						rg.SetSelected("Default (~/FtRSync/user/repo)")
					}
					pathEntry := widget.NewEntry()
					pathEntry.SetText(e.SyncCustomPath)
					dialog.ShowCustomConfirm("Edit Auto Sync Entry", "Save", "Cancel",
						container.NewVBox(
							widget.NewLabel(fmt.Sprintf("Editing: %s/%s", e.User, e.Repo)),
							rg,
							container.NewBorder(nil, nil, nil, widget.NewButton("Select", func() {
								dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
									if err == nil && uri != nil {
										pathEntry.SetText(uri.Path())
									}
								}, w)
							}), pathEntry),
						),
						func(confirm bool) {
							if confirm {
								// persist changes
								newMode := "Default"
								switch rg.Selected {
								case "Always Sync To...":
									newMode = "Custom"
								case "Ask Every Time":
									newMode = "Ask"
								}
								appSettings.AutoSyncEntries[idx].SyncMode = newMode
								appSettings.AutoSyncEntries[idx].SyncCustomPath = pathEntry.Text
								saveSettings()
								autoSyncList.Refresh()
							}
						}, w)
				}
			}
		},
	)
	autoSyncList.OnSelected = func(id widget.ListItemID) { autoSyncSelected = id }

	addAutoSyncBtn := widget.NewButton("Add Repo...", func() {
		userEntry := widget.NewEntry()
		repoEntry := widget.NewEntry()
		dialog.ShowForm("Add Auto Sync Repo", "Add", "Cancel",
			[]*widget.FormItem{widget.NewFormItem("User", userEntry), widget.NewFormItem("Repo", repoEntry)},
			func(ok bool) {
				if ok {
					u := strings.TrimSpace(userEntry.Text)
					r := strings.TrimSpace(repoEntry.Text)
					if u != "" && r != "" {
						appSettings.AutoSyncEntries = append(appSettings.AutoSyncEntries, AutoSyncEntry{User: u, Repo: r, SyncMode: appSettings.SyncMode, SyncCustomPath: appSettings.SyncCustomPath, ShowReceipt: false})
						saveSettings()
						autoSyncList.Refresh()
					}
				}
			}, w)
	})

	removeAutoSyncBtn := widget.NewButton("Remove Selected", func() {
		if autoSyncSelected >= 0 && int(autoSyncSelected) < len(appSettings.AutoSyncEntries) {
			idx := int(autoSyncSelected)
			appSettings.AutoSyncEntries = append(appSettings.AutoSyncEntries[:idx], appSettings.AutoSyncEntries[idx+1:]...)
			saveSettings()
			autoSyncList.Refresh()
		}
	})

	syncAllBtn := widget.NewButton("Sync All", func() {
		syncAllEntries()
		if appSettings.AutoSyncIntervalSeconds > 0 {
			select {
			case resetCountdown <- appSettings.AutoSyncIntervalSeconds:
			default:
				select {
				case <-resetCountdown:
				default:
				}
				resetCountdown <- appSettings.AutoSyncIntervalSeconds
			}
		}
		uiQueue <- func() {
			countdownLabel.SetText(fmt.Sprintf("Next sync: %s", formatTime(appSettings.AutoSyncIntervalSeconds)))
		}
	})

	tabs := container.NewAppTabs(
		container.NewTabItem("Search", searchTabContent),
		container.NewTabItem("Login", loginForm),
		container.NewTabItem("Account", accountTabContent),
		container.NewTabItem("Browser", container.NewHSplit(
			container.NewBorder(repoListLabel, nil, nil, nil, repoList),
			browserRightPane,
		)),
		container.NewTabItem("Auto Sync", container.NewBorder(autoSyncLabel, container.NewHBox(addAutoSyncBtn, removeAutoSyncBtn, syncAllBtn), nil, nil, autoSyncList)),
		container.NewTabItem("Settings", settingsTabContent),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	// UI Update Logic
	updateUI = func() {
		log.Println("Updating UI based on login state...")
		if ftrClient.IsLoggedIn() {
			email, username := ftrClient.GetSessionInfo()
			loggedInMsg := fmt.Sprintf("Logged in as %s (%s)", username, email)
			loginStatusLabel.SetText(loggedInMsg)
			accountStatusLabel.SetText(loggedInMsg)
			loginForm.Hide()
			tabs.EnableItem(tabs.Items[3])
			if tabs.SelectedIndex() != 2 {
				tabs.SelectIndex(2)
			}
			tabs.SelectIndex(2)
			go func(u string) {
				log.Printf("Fetching repositories for user: %s", u)
				matches, err := ftrClient.SearchRepos(u)
				if err != nil {
					uiQueue <- func() { dialog.ShowError(err, w) }
					return
				}
				repos := []map[string]string{}
				for _, m := range matches {
					if m["user"] == u {
						repos = append(repos, m)
					}
				}
				uiQueue <- func() {
					userRepos = repos
					repoList.Refresh()
				}
			}(username)
		} else {
			tabs.DisableItem(tabs.Items[3])
			loginStatusLabel.SetText("You are not logged in. Enter your InkDrop credentials to log in.")
			accountStatusLabel.SetText("Not logged in.")
			loginForm.Show()
			userRepos = []map[string]string{}
			repoFiles = []map[string]interface{}{}
			fileList.Refresh()
			tabs.SelectIndex(1)
			repoList.Refresh()
		}
	}

	loginButton.OnTapped = func() {
		go func() {
			log.Printf("Login button clicked. Attempting login for user: %s", emailEntry.Text)
			loggingInMsg := dialog.NewCustomWithoutButtons("Logging in...", widget.NewLabel("Please wait."), w)
			uiQueue <- func() { loggingInMsg.Show() }
			err := ftrClient.Login(emailEntry.Text, passwordEntry.Text)
			if err != nil {
				uiQueue <- loggingInMsg.Hide
				uiQueue <- func() { dialog.ShowError(err, w) }
				return
			}

			uiQueue <- func() {
				uiQueue <- func() { loggingInMsg.Hide() }
				dialog.ShowInformation("Success", "Successfully logged in.", w)
				updateUI()
			}
		}()
	}

	logoutButton.OnTapped = func() {
		go func() {
			log.Println("Logout button clicked.")
			if err := ftrClient.Logout(); err != nil {
				uiQueue <- func() { dialog.ShowError(err, w) }
			}
			uiQueue <- updateUI
		}()
	}

	go func() {
		for fn := range uiQueue {
			fyne.Do(fn)
		}
	}()

	// background auto-sync countdown worker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		debugMode := os.Getenv("FTR_DEBUG") != ""
		var remaining int
		if appSettings.AutoSyncIntervalSeconds > 0 {
			remaining = appSettings.AutoSyncIntervalSeconds
		} else if debugMode {
			remaining = 20
		}
		for {
			select {
			case newInterval := <-resetCountdown:
				remaining = newInterval
				uiQueue <- func() {
					countdownLabel.SetText(fmt.Sprintf("Next sync: %s", formatTime(remaining)))
				}
			case <-ticker.C:
				interval := appSettings.AutoSyncIntervalSeconds
				if interval <= 0 {
					if debugMode {
						// keep a short debug countdown even when interval is 0
						interval = 20
					} else {
						// disabled
						continue
					}
				}
				if remaining <= 0 {
					if appSettings.AutoSyncIntervalSeconds > 0 {
						remaining = appSettings.AutoSyncIntervalSeconds
					} else if debugMode {
						remaining = 20
					} else {
						continue
					}
				}
				// update visible countdown label
				uiQueue <- func() {
					countdownLabel.SetText(fmt.Sprintf("Next sync: %s", formatTime(remaining)))
				}
				remaining--
				if remaining <= 0 {
					log.Printf("Auto-sync: interval reached, triggering Sync All")
					syncAllEntries()
					if appSettings.AutoSyncIntervalSeconds > 0 {
						remaining = appSettings.AutoSyncIntervalSeconds
					} else if debugMode {
						remaining = 20
					}
				}
			}
		}
	}()

	w.Resize(fyne.NewSize(float32(appWidth), float32(appHeight)))

	var mainLayout *fyne.Container

	if runtime.GOOS != "darwin" {
		closeBtn := widget.NewButtonWithIcon("", theme.WindowCloseIcon(), func() {
			w.Close()
		})

		title := widget.NewLabel(appName)
		title.TextStyle.Bold = true

		titleBar := container.NewGridWithColumns(3,
			// Left side: spacer
			widget.NewLabel(""),
			// Center: Title
			container.NewCenter(title),
			// Right side: window controls
			container.NewHBox(layout.NewSpacer(), closeBtn),
		)

		dragArea := canvas.NewRectangle(color.Transparent)
		draggableTitleBar := container.NewStack(titleBar, dragArea)
		mainLayout = container.NewBorder(
			draggableTitleBar,
			nil, nil, nil, // bottom, left, right
			tabs,
		)
	} else {
		mainLayout = container.NewBorder(
			nil,
			nil, nil, nil, // bottom, left, right
			tabs,
		)
	}

	// --- Window Layout ---

	tabs.DisableItem(tabs.Items[3])
	updateUI()

	w.SetMaster()
	w.SetContent(mainLayout)
	w.SetFixedSize(false)
	w.SetPadded(false)
	w.ShowAndRun()
}

func startDownload(user, repo, fileName, destPath string) {
	go func() {
		progress := widget.NewProgressBar()
		progressDialog := dialog.NewCustomWithoutButtons(
			"Downloading...",
			container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Downloading %s...", fileName)),
				progress,
			),
			w,
		)
		uiQueue <- func() { progressDialog.Show() }

		err := ftrClient.DownloadAndVerify(user, repo, fileName, destPath, func(p float64) {
			uiQueue <- func() { progress.SetValue(p) }
		})

		done := make(chan struct{})
		uiQueue <- func() {
			progressDialog.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("download failed: %w", err), w)
			} else {
				dialog.ShowInformation("Success", "File downloaded successfully.", w)
			}
			close(done)
		}
		<-done
	}()
}

func showDeleteConfirm(fileName, user, repo string, w fyne.Window, client *api.Client, uiQueue chan func(), onComplete func()) {
	dialog.ShowConfirm("Delete File?", fmt.Sprintf("Are you sure you want to delete '%s' from the repository?", fileName), func(confirm bool) {
		if !confirm {
			return
		}
		go func() {
			log.Printf("Attempting to delete %s from %s/%s", fileName, user, repo)
			err := client.DeleteRemoteFile(user, repo, fileName)
			uiQueue <- func() {
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to delete file: %w", err), w)
					return
				}
				dialog.ShowInformation("Success", "File deleted successfully.", w)
				if onComplete != nil {
					onComplete() // Refresh the file list
				}
			}
		}()
	}, w)
}

// performSync performs a sync for a repository with confirmation summary.
// Minimal stub for initial compilation; will be expanded to list remote/local diffs and perform transfers.
// performSync performs a sync for a repository with confirmation summary.
func performSync(user, repo, mode, customPath string, showReceipt bool) {
	log.Printf("performSync: %s/%s mode=%s receipt=%v", user, repo, mode, showReceipt)

	var syncDir string
	var err error

	// Resolve sync directory based on mode
	switch mode {
	case "Ask":
		// Show folder chooser on UI thread and wait
		ch := make(chan struct{})
		uiQueue <- func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, errDialog error) {
				if errDialog != nil {
					err = fmt.Errorf("dialog error: %w", errDialog)
				} else if uri == nil {
					err = fmt.Errorf("sync cancelled by user")
				} else {
					syncDir = uri.Path()
				}
				close(ch)
			}, w)
		}
		<-ch
	case "Custom":
		syncDir = customPath
		if syncDir == "" {
			err = fmt.Errorf("custom sync path is not set")
		}
	default:
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			err = fmt.Errorf("failed to determine home directory: %w", homeErr)
		} else {
			syncDir = filepath.Join(home, "FtRSync", user, repo)
		}
	}

	if err != nil {
		if err.Error() != "sync cancelled by user" {
			uiQueue <- func() { dialog.ShowError(err, w) }
		}
		return
	}
	if syncDir == "" {
		log.Printf("performSync: no sync directory for %s/%s", user, repo)
		return
	}

	// List remote files
	remoteFiles, err := ftrClient.ListRepoFiles(user, repo)
	if err != nil {
		uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to list remote files: %w", err), w) }
		return
	}

	// Ensure sync directory exists
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to create sync directory: %w", err), w) }
		return
	}

	// Scan local files
	localFiles := make(map[string]LocalFileInfo)
	err = filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(syncDir, path)
			rel = filepath.ToSlash(rel)
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
			localFiles[rel] = LocalFileInfo{Info: info, Hash: fmt.Sprintf("%x", h.Sum(nil))}
		}
		return nil
	})
	if err != nil {
		uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to scan local files: %w", err), w) }
		return
	}

	// Compare
	uploads := []string{}
	downloads := []string{}
	conflicts := []string{}
	remoteMap := make(map[string]map[string]interface{})
	for _, rf := range remoteFiles {
		if p, ok := rf["path"].(string); ok {
			remoteMap[p] = rf
		}
	}
	for lp, lf := range localFiles {
		if rf, ok := remoteMap[lp]; !ok {
			uploads = append(uploads, lp)
		} else {
			if rh, ok := rf["hash"].(string); ok && rh != lf.Hash {
				conflicts = append(conflicts, lp)
			}
		}
	}
	for rp := range remoteMap {
		if _, ok := localFiles[rp]; !ok {
			downloads = append(downloads, rp)
		}
	}
	downloads = append(downloads, conflicts...)

	// compute sizes
	var totalUploadSize, totalDownloadSize int64
	for _, p := range uploads {
		if lf, ok := localFiles[p]; ok {
			totalUploadSize += lf.Info.Size()
		}
	}
	for _, p := range downloads {
		if rf, ok := remoteMap[p]; ok {
			if s, ok := rf["size"].(float64); ok {
				totalDownloadSize += int64(s)
			}
		}
	}

	// detailed confirmation with exact file lists
	if showReceipt {
		okCh := make(chan bool)
		uiQueue <- func() {
			header := widget.NewLabel(fmt.Sprintf("About to sync %s/%s:\nUploads: %d (%.2f MB)  Downloads: %d (%.2f MB)",
				user, repo, len(uploads), float64(totalUploadSize)/1024.0/1024.0, len(downloads), float64(totalDownloadSize)/1024.0/1024.0))

			uploadText := "(none)"
			if len(uploads) > 0 {
				uploadText = strings.Join(uploads, "\n")
			}
			downloadText := "(none)"
			if len(downloads) > 0 {
				downloadText = strings.Join(downloads, "\n")
			}

			uploadEntry := widget.NewMultiLineEntry()
			uploadEntry.SetText(uploadText)
			uploadEntry.Disable()
			uploadEntry.SetMinRowsVisible(12)
			downloadEntry := widget.NewMultiLineEntry()
			downloadEntry.SetText(downloadText)
			downloadEntry.Disable()
			downloadEntry.SetMinRowsVisible(12)

			content := container.NewVBox(
				header,
				widget.NewLabelWithStyle("Files to Upload:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				container.NewVScroll(uploadEntry),
				widget.NewLabelWithStyle("Files to Download:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				container.NewVScroll(downloadEntry),
			)

			dialog.ShowCustomConfirm("Sync Confirmation", "Sync", "Cancel", content, func(ok bool) { okCh <- ok }, w)
		}
		if !<-okCh {
			uiQueue <- func() { dialog.ShowInformation("Cancelled", "Sync cancelled by user.", w) }
			return
		}
	}

	// Start Sync Logic with Progress
	statusLabel := widget.NewLabel("Starting sync...")
	overallProgress := widget.NewProgressBar()

	workerContainer := container.NewVBox()
	workerBars := make([]*widget.ProgressBar, guiWorkers)
	workerLabels := make([]*widget.Label, guiWorkers)
	for i := 0; i < guiWorkers; i++ {
		workerLabels[i] = widget.NewLabel(fmt.Sprintf("Worker %d: Idle", i+1))
		workerBars[i] = widget.NewProgressBar()
		workerContainer.Add(workerLabels[i])
		workerContainer.Add(workerBars[i])
	}

	progressDialog := dialog.NewCustomWithoutButtons("Synchronising...", container.NewVBox(statusLabel, overallProgress, widget.NewSeparator(), workerContainer), w)
	uiQueue <- func() { progressDialog.Show() }

	totalSyncSize := totalDownloadSize + totalUploadSize
	var syncedSize int64

	type syncTask struct {
		op   string // "download" or "upload"
		path string
		size int64
	}
	taskChan := make(chan syncTask, len(downloads)+len(uploads))
	for _, path := range downloads {
		size := 0.0
		if rf, ok := remoteMap[path]; ok {
			size, _ = rf["size"].(float64)
		}
		taskChan <- syncTask{op: "download", path: path, size: int64(size)}
	}
	for _, path := range uploads {
		size := int64(0)
		if lf, ok := localFiles[path]; ok {
			size = lf.Info.Size()
		}
		taskChan <- syncTask{op: "upload", path: path, size: size}
	}
	close(taskChan)

	var wg sync.WaitGroup
	for i := 0; i < guiWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskChan {
				uiQueue <- func() {
					workerLabels[workerID].SetText(fmt.Sprintf("%s: %s", strings.Title(task.op), task.path))
					workerBars[workerID].SetValue(0)
				}

				var lastP float64
				progressCb := func(p float64) {
					deltaP := p - lastP
					lastP = p
					deltaBytes := int64(deltaP * float64(task.size))
					newTotal := atomic.AddInt64(&syncedSize, deltaBytes)

					uiQueue <- func() {
						workerBars[workerID].SetValue(p)
						if totalSyncSize > 0 {
							overallProgress.SetValue(float64(newTotal) / float64(totalSyncSize))
						}
					}
				}

				if task.op == "download" {
					destPath := filepath.Join(syncDir, filepath.FromSlash(task.path))
					os.MkdirAll(filepath.Dir(destPath), 0755)
					if err := ftrClient.DownloadAndVerify(user, repo, task.path, destPath, progressCb); err != nil {
						log.Printf("download failed: %v", err)
					}
				} else {
					localPath := filepath.Join(syncDir, filepath.FromSlash(task.path))
					if file, err := os.Open(localPath); err == nil {
						repoPath := fmt.Sprintf("%s/%s", user, repo)
						if err := ftrClient.UploadFile(repoPath, task.path, file, task.size, false, progressCb); err != nil {
							log.Printf("upload failed: %v", err)
						}
						file.Close()
					}
				}

				uiQueue <- func() {
					workerLabels[workerID].SetText(fmt.Sprintf("Worker %d: Idle", workerID+1))
					workerBars[workerID].SetValue(0)
				}
			}
		}(i)
	}
	wg.Wait()

	uiQueue <- func() {
		progressDialog.Hide()
		dialog.ShowInformation("Sync Complete", fmt.Sprintf("Finished syncing %s/%s.", user, repo), w)
	}
}

func fileInfoSize(f *os.File) int64 {
	if fi, err := f.Stat(); err == nil {
		return fi.Size()
	}
	return 0
}

func formatTime(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func syncAllEntries() {
	log.Printf("syncAllEntries: running %d entries", len(appSettings.AutoSyncEntries))
	for _, e := range appSettings.AutoSyncEntries {
		go performSync(e.User, e.Repo, e.SyncMode, e.SyncCustomPath, e.ShowReceipt)
	}
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "inker", "settings.json")
}

func loadSettings(a fyne.App) {
	// Set defaults
	home, _ := os.UserHomeDir()
	appSettings.Theme = "System"
	appSettings.DownloadAsk = true
	appSettings.downloadPathM = filepath.Join(home, "Downloads")
	appSettings.SyncMode = "Default"
	appSettings.SyncCustomPath = filepath.Join(home, "FtRSync")

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading settings file: %v", err)
		}
		return
	}

	if err := json.Unmarshal(data, &appSettings); err != nil {
		log.Printf("Error parsing settings file: %v", err)
	}

	// Post-load adjustments
	if appSettings.downloadPathM == "" {
		appSettings.downloadPathM = filepath.Join(home, "Downloads")
	}
	if appSettings.SyncMode == "" {
		appSettings.SyncMode = "Default"
	}

	// Backwards compatibility: support both minutes and seconds stored in older/newer versions.
	if appSettings.AutoSyncIntervalSeconds > 0 {
		// populate minutes for UI convenience
		appSettings.AutoSyncIntervalMinutes = appSettings.AutoSyncIntervalSeconds / 60
	} else if appSettings.AutoSyncIntervalMinutes > 0 {
		appSettings.AutoSyncIntervalSeconds = appSettings.AutoSyncIntervalMinutes * 60
	}

	applyTheme(a)
}

func saveSettings() {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Printf("Error creating settings directory: %v", err)
		return
	}

	data, err := json.MarshalIndent(appSettings, "", "  ")
	if err != nil {
		log.Printf("Error marshalling settings: %v", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Error writing settings file: %v", err)
	}
}

func applyTheme(a fyne.App) {
	switch appSettings.Theme {
	case "Light":
		a.Settings().SetTheme(theme.LightTheme())
	case "Dark":
		a.Settings().SetTheme(theme.DarkTheme())
	default:
		a.Settings().SetTheme(theme.DefaultTheme())
	}
}
