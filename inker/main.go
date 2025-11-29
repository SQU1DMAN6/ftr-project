package main

import (
	"fmt"
	"image/color"
	"inker/api"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	appName   = "FtR Inker"
	appWidth  = 1200
	appHeight = 900
)

func main() {
	// Channel to queue UI updates from brackground goroutines
	uiQueue := make(chan func(), 100)

	a := app.New()

	w := a.NewWindow(appName)

	ftrClient, err := api.NewClient()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to establish connection with InkDrop server: %w", err), w)
	}

	var updateUI func()

	var searchResults []map[string]string
	resultsList := widget.NewList(
		func() int {
			return len(searchResults)
		},
		func() fyne.CanvasObject {
			buttonBox := container.NewHBox(
				widget.NewButton("Install", nil),
				widget.NewButton("Down", nil),
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
							getBtn.OnTapped = func() {
								dialog.ShowInformation("Get", "Get functionality is soon to come.", w)
							}
							downBtn.OnTapped = func() {
								dialog.ShowInformation("Down", "Down functionality is soon to come.", w)
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
			tabs.Items[3].Content = accountTabContent
		} else {
			loginStatusLabel.SetText("Enter your InkDrop credentials to log in.")
			accountStatusLabel.SetText("You are not logged in.")
			loginForm.Show()
			tabs.Items[3].Content = container.NewMax()
		}
		tabs.Refresh()
	}

	loginButton.OnTapped = func() {
		go func() {
			log.Printf("Login button clicked. Attempting login for user: %s", emailEntry.Text)
			err := ftrClient.Login(emailEntry.Text, passwordEntry.Text)
			if err != nil {
				uiQueue <- func() {
					dialog.ShowError(err, w)
				}
				return
			}
			uiQueue <- func() {
				dialog.ShowInformation("Success", "Successfully logged in.", w)
				updateUI()
				tabs.SelectIndex(3)
			}
		}()
	}

	logoutButton.OnTapped = func() {
		go func() {
			log.Println("Logout button clicked.")
			if err := ftrClient.Logout(); err != nil {
				uiQueue <- func() {
					dialog.ShowError(err, w)
				}
			}
			uiQueue <- func() {
				updateUI()
				tabs.SelectIndex(1)
			}
		}()
	}

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

	// Custom drag handler for the title bar
	dragArea := canvas.NewRectangle(color.Transparent)
	draggableTitleBar := container.NewStack(titleBar, dragArea)

	// --- Window Layout ---
	mainLayout := container.NewBorder(
		draggableTitleBar,
		nil, nil, nil, // bottom, left, right
		tabs,
	)

	w.SetMaster()
	w.SetContent(mainLayout)
	w.CenterOnScreen()
	w.SetFixedSize(false)
	w.SetPadded(false)
	w.ShowAndRun()
}
