package main

import (
	"fmt"
	"image/color"
	"inker/api"
	"inker/builder"
	"inker/fsdl"
	"log"
	"os"
	"path/filepath"

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
	appName   = "FtR Inker"
	appWidth  = 800
	appHeight = 600
)

var updateUI func()

func main() {
	// Channel to queue UI updates from brackground goroutines
	uiQueue := make(chan func(), 100)

	a := app.NewWithID("0")

	var w fyne.Window

	// Destination directory
	var dest string
	var downDest string

	if drv, ok := a.Driver().(desktop.Driver); ok {
		w = drv.CreateSplashWindow()
	} else {
		w = a.NewWindow(appName)
	}

	// w = a.NewWindow(appName)

	ftrClient, err := api.NewClient()
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
									progressLabel := widget.NewLabel("Preparing to install...")
									progressOverall := widget.NewProgressBar()
									progressFile := widget.NewProgressBarInfinite()
									progressDialog := dialog.NewCustomWithoutButtons("Installing...", container.NewVBox(progressLabel, progressOverall, progressFile), w)
									uiQueue <- func() { progressDialog.Show() }
									defer func() { uiQueue <- func() { progressDialog.Hide() } }()

									repoPath := fmt.Sprintf("%s/%s", user, repo)
									tmpDir := "/tmp/fsdl"
									if err := os.MkdirAll(tmpDir, 0755); err != nil {
										log.Printf("failed to create temp directory: %v", err)
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to create temp directory: %w", err), w) }
										return
									}

									fsdlFile := filepath.Join(tmpDir, repo+".fsdl")

									// Download from server
									log.Printf("Fetching repo %s", repoPath)
									uiQueue <- func() {
										progressLabel.SetText(fmt.Sprintf("Downloading metadata for %s", repoPath))
										progressOverall.SetValue(0.1)
									}

									log.Println("Fetching package via API...")
									// Use repo.php API to download and verify
									if err := ftrClient.DownloadAndVerify(user, repo, repo+".fsdl", fsdlFile); err != nil {
										log.Printf("download failed: %v", err)
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("metadata download failed: %w", err), w) }
										return
									}

									uiQueue <- func() {
										progressLabel.SetText("Extracting package...")
										progressOverall.SetValue(0.3)
									}
									if err := fsdl.Extract(fsdlFile, tmpDir); err != nil {
										log.Printf("failed to extract package: %v", err)
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("failed to extract package: %w", err), w) }
										return
									}

									b := builder.New(repo, tmpDir)

									binaryPath, err := b.DetectAndBuild()
									if err != nil {
										log.Printf("build failed: %v", err)
										uiQueue <- func() { dialog.ShowError(fmt.Errorf("build failed: %w", err), w) }
										return
									}

									if binaryPath != "" {
										uiQueue <- func() {
											progressLabel.SetText("Installing binary (requires privileges)...")
											progressOverall.SetValue(0.8)
										}
										if err := b.InstallBinary(binaryPath); err != nil {
											log.Printf("installation failed: %v", err)
											uiQueue <- func() { dialog.ShowError(fmt.Errorf("installation failed: %w", err), w) }
											return
										}
									}

									uiQueue <- func() {
										progressOverall.SetValue(1.0)
										progressFile.Hide()
										dialog.ShowInformation("Install complete", fmt.Sprintf("Finished installing %s/%s", user, repo), w)
									}
								}(user, repo)
							}

							// Download button event handling - download all files in a repository
							downBtn.OnTapped = func() {
								info := fmt.Sprintf("User: %s, Repository: %s", user, repo)
								log.Printf("Download button clicked for: %s", info)

								go func(user, repo string) {
									// Check if the client is valid before proceeding
									if ftrClient == nil {
										log.Println("Download failed: client is not initialized.")
										uiQueue <- func() {
											dialog.ShowError(fmt.Errorf("client is not initialized, cannot download files"), w)
										}
										return
									}

									progressLabel := widget.NewLabel("Preparing to download...")
									progressOverall := widget.NewProgressBar()
									progressFile := widget.NewProgressBarInfinite()
									progressDialog := dialog.NewCustomWithoutButtons("Downloading...", container.NewVBox(progressLabel, progressOverall, progressFile), w)

									uiQueue <- func() { progressDialog.Show() }
									defer func() { uiQueue <- func() { progressDialog.Hide() } }()

									if downDest != "" {
										dest = downDest
									} else {
										home, err := os.UserHomeDir()
										if err != nil {
											log.Printf("failed to determine home directory: %v", err)
											uiQueue <- func() {
												dialog.ShowError(fmt.Errorf("failed to determine user's home directory: %v", err), w)
											}
											return
										}
										dest = filepath.Join(home, "FtRSync", user, repo)
									}

									if err := os.MkdirAll(dest, 0755); err != nil {
										log.Printf("failed to create destination directory: %v", err)
										uiQueue <- func() {
											dialog.ShowError(fmt.Errorf("failed to create destination directory: %v", err), w)
										}
										return
									}

									log.Printf("Listing files in %s/%s...", user, repo)
									files, err := ftrClient.ListRepoFiles(user, repo)
									if err != nil {
										log.Printf("failed to list repository files: %v", err)
										uiQueue <- func() {
											dialog.ShowError(fmt.Errorf("failed to list repository files: %w", err), w)
										}
										return
									}

									if len(files) == 0 {
										log.Println("No files were found in the repository.")
										uiQueue <- func() {
											dialog.ShowInformation("Empty Repository", "No files were found in the repository.", w)
										}
										return
									}

									errorsList := []string{}

									totalFiles := len(files)
									for i, f := range files {
										pathRel, _ := f["path"].(string)
										if pathRel == "" {
											continue
										}

										uiQueue <- func() {
											progressLabel.SetText(fmt.Sprintf("Downloading: %s", pathRel))
											progressOverall.SetValue(float64(i) / float64(totalFiles))
										}

										fullPath := filepath.Join(dest, filepath.FromSlash(pathRel))
										if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
											errorsList = append(errorsList, fmt.Sprintf("failed to create dir for %s: %v", fullPath, err))
											continue
										}

										// Start download
										if ftrClient == nil {
											log.Println("Download cancelled: client is no longer valid.")
											uiQueue <- func() {
												dialog.ShowError(fmt.Errorf("client became invalid during download, possibly due to logout"), w)
											}
											return
										}
										if err := ftrClient.DownloadAndVerify(user, repo, pathRel, fullPath); err != nil {
											errorsList = append(errorsList, fmt.Sprintf("failed to download %s: %v", pathRel, err))
											continue
										}
									}

									uiQueue <- func() {
										progressOverall.SetValue(1.0)
										progressFile.Hide()
										if len(errorsList) > 0 {
											log.Printf("Errors encountered during download for %s/%s:\n%v", user, repo, errorsList)
											dialog.ShowInformation("Encountered errors trying to download %s/%s", fmt.Sprintf("Errors: %v", errorsList), w)
										}
										dialog.ShowInformation("Download Complete", fmt.Sprintf("Finished downloading %s/%s", user, repo), w)
										log.Println("All files processed.")
									}
								}(user, repo)
							}

							// Sync button does the same as Download for now.
							syncBtn.OnTapped = downBtn.OnTapped

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

	var repoList *widget.List // Declare repoList early

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

			downloadBtn := box.Objects[2].(*widget.Button)
			downloadBtn.OnTapped = func() {
				dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
					if writer == nil {
						return // User cancelled
					}
					defer writer.Close()
					go func() {
						progress := dialog.NewProgressInfinite("Downloading", fmt.Sprintf("Downloading %s...", fileName), w)
						progress.Show()
						defer progress.Hide()

						err := ftrClient.DownloadAndVerify(selectedRepoUser, selectedRepo, fileName, writer.URI().Path())
						if err != nil {
							uiQueue <- func() { dialog.ShowError(fmt.Errorf("download failed: %w", err), w) }
							return
						}
						uiQueue <- func() { dialog.ShowInformation("Success", "File downloaded successfully.", w) }
					}()
				}, w)
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
			selectedRepo = selectedRepoName // The repo name, not the full path
			selectedRepoLabel.SetText(fmt.Sprintf("Selected Repo: %s", selectedRepoName))
			updateUploadButtonState()

			// Fetch and display files for the selected repo
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
			// Capture state for the goroutine
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
			defer func() { uiQueue <- func() { progressDialog.Hide() } }()

			uploadErr := ftrClient.UploadFile(repoToUpload, fileName, fileReader, fileSize, encryptUpload, func(progress float64) {
				uiQueue <- func() { uploadProgress.SetValue(progress) }
			})
			if uploadErr != nil {
				log.Printf("Upload failed: %v", err)
				uiQueue <- func() { dialog.ShowError(fmt.Errorf("upload failed: %w", uploadErr), w) }
				return
			} else {
				if repoList != nil {
					repoList.OnSelected(selectedRepoID) // Refresh file list
				}
			}

			uiQueue <- func() {
				dialog.ShowInformation("Success", fmt.Sprintf("Successfully uploaded %s to %s", fileName, selectedRepo), w)
			}
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

			// Fetch user's repos
			go func(u string) {
				log.Printf("Fetching repositories for user: %s", u)
				matches, err := ftrClient.SearchRepos(u)
				if err != nil {
					uiQueue <- func() { dialog.ShowError(err, w) }
					return
				}
				// Filter to only show repos owned by the user
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
				loggingInMsg.Hide()
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
