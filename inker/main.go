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

	a := app.New()

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
					if len(v.Objects) == 2 {
						if _, ok := v.Objects[0].(*widget.Button); ok {
							getBtn := v.Objects[0].(*widget.Button)
							downBtn := v.Objects[1].(*widget.Button)

							// Get button event handling - download .fsdl file in a repository and install it to the user's system
							getBtn.OnTapped = func() {
								info := fmt.Sprintf("User: %s, Repository: %s", user, repo)
								log.Printf("Install button clicked for: %s", info)

								go func(user, repo string) {
									// dialog.ShowInformation("Install", info, w)
									repoPath := fmt.Sprintf("%s/%s", user, repo)
									tmpDir := "/tmp/fsdl"
									if err := os.MkdirAll(tmpDir, 0755); err != nil {
										log.Fatalf("failed to create temp directory: %v", err)
									}

									fsdlFile := filepath.Join(tmpDir, repo+".fsdl")

									// Download from server
									log.Printf("Fetching repo %s", repoPath)

									// Try to fetch repository description to show the user
									if matches, err := ftrClient.SearchRepos(repo); err == nil {
										for _, m := range matches {
											if m["user"] == user && m["repo"] == repo {
												desc := m["description"]
												if desc == "" {
													desc = "(no description)"
												}
												log.Printf("Description: %s", desc)
												break
											}
										}
									}

									log.Println("Fetching package via API...")
									// Use repo.php API to download and verify
									if err := ftrClient.DownloadAndVerify(user, repo, repo+".fsdl", fsdlFile); err != nil {
										log.Fatalf("download failed: %v", err)
										return
									}

									if err := fsdl.Extract(fsdlFile, tmpDir); err != nil {
										log.Fatalf("failed to extract package: %v", err)
										return
									}

									b := builder.New(repo, tmpDir)

									binaryPath, err := b.DetectAndBuild()
									if err != nil {
										log.Fatalf("build failed: %v", err)
										dialog.ShowError(fmt.Errorf("build failed: %w", err), w)
									}

									if binaryPath != "" {
										if err := b.InstallBinary(binaryPath); err != nil {
											log.Fatalf("installation failed: %v", err)
											dialog.ShowError(fmt.Errorf("installation failed: %w", err), w)
										}
									}

									uiQueue <- func() {
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
									progressBar := widget.NewProgressBar()
									progressDialog := dialog.NewCustomWithoutButtons("Downloading...", container.NewVBox(progressLabel, progressBar), w)

									uiQueue <- func() { progressDialog.Show() }

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
											progressBar.SetValue(float64(i) / float64(totalFiles))
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
										progressDialog.Hide()
										if len(errorsList) > 0 {
											log.Printf("Errors encountered during download for %s/%s:\n%v", user, repo, errorsList)
											dialog.ShowInformation("Encountered errors trying to download %s/%s", fmt.Sprintf("Errors: %v", errorsList), w)
										}
										dialog.ShowInformation("Download Complete", fmt.Sprintf("Finished downloading %s/%s", user, repo), w)
										log.Println("All files processed.")
									}
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
	content := container.NewMax(placeholder, resultsList)

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

	loginStatusLabel := widget.NewLabel("You are not logged in.")
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

	// Rendering App tabs

	tabs := container.NewAppTabs(
		container.NewTabItem("Search", searchTabContent),
		container.NewTabItem("Login", loginForm),
		container.NewTabItem("Account", accountTabContent),
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
			tabs.Items[2].Content = accountTabContent
			tabs.SelectIndex(2)
		} else {
			loginStatusLabel.SetText("Enter your InkDrop credentials to log in.")
			accountStatusLabel.SetText("You are not logged in.")
			loginForm.Show()
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

	updateUI()

	w.SetMaster()
	w.SetContent(mainLayout)
	w.SetFixedSize(false)
	w.CenterOnScreen()
	w.SetPadded(false)
	w.ShowAndRun()
}
