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
	"strings"
	"sync"
	"sync/atomic"

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
	Theme         string `json:"theme"`
	DownloadPath  string `json:"download_path"`
	DownloadAsk   bool   `json:"download_ask"`
	SyncPath      string `json:"sync_path"`
	SyncAsk       bool   `json:"sync_ask"`
	downloadPathM string
	syncPathM     string
}

type LocalFileInfo struct {
	Info os.FileInfo
	Hash string
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

	a := app.NewWithID("0")
	loadSettings(a)

	// Destination directory
	var downDest string

	if drv, ok := a.Driver().(desktop.Driver); ok {
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
										if appSettings.SyncAsk {
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
										} else {
											baseSyncPath := appSettings.syncPathM
											if baseSyncPath == "" {
												home, _ := os.UserHomeDir()
												baseSyncPath = filepath.Join(home, "FtRSync")
											}
											syncDir = filepath.Join(baseSyncPath, user, repo)
											close(askDone)
										}
									}

									<-askDone

									if err != nil {
										log.Println(err)
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

	tabs := container.NewAppTabs(
		container.NewTabItem("Search", searchTabContent),
		container.NewTabItem("Login", loginForm),
		container.NewTabItem("Account", accountTabContent),
		container.NewTabItem("Browser", container.NewHSplit(
			container.NewBorder(repoListLabel, nil, nil, nil, repoList),
			browserRightPane,
		)),
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

	w.Resize(fyne.NewSize(float32(appWidth), float32(appHeight)))

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

	// --- Window Layout ---
	mainLayout := container.NewBorder(
		draggableTitleBar,
		nil, nil, nil, // bottom, left, right
		tabs,
	)

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
	appSettings.SyncAsk = true
	appSettings.syncPathM = filepath.Join(home, "FtRSync")

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
	if appSettings.syncPathM == "" {
		appSettings.syncPathM = filepath.Join(home, "FtRSync")
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
