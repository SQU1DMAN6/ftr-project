package main

import (
	"fmt"
	"image/color"
	"inker/api"
	"log"
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

func main() {
	a := app.New()

	// --- Create a borderless splash window ---
	drv := a.Driver()
	var w fyne.Window
	if drv, ok := drv.(desktop.Driver); ok {
		w = drv.CreateSplashWindow()
	} else {
		// Fallback for mobile or other non-desktop drivers
		w = a.NewWindow(appName)
	}

	// --- API Client ---
	ftrClient, err := api.NewClient()
	if err != nil {
		log.Fatalf("Failed to create API client: %v", err)
	}

	// --- Get Logic ---
	getFunc := func(user, repo string) {
		files, err := ftrClient.ListRepoFiles(user, repo)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if len(files) == 0 {
			dialog.ShowInformation("Empty Repository", "This repository contains no files.", w)
			return
		}

		// For simplicity, we'll just handle the first file. A real implementation might show a list.
		fileToGet, ok := files[0]["name"].(string)
		if !ok {
			dialog.ShowError(fmt.Errorf("invalid file data in repository"), w)
			return
		}

		destPath := filepath.Join("/usr/local/bin", fileToGet) // Example path that may require sudo
		message := fmt.Sprintf("This will download '%s' from '%s/%s' to '%s'.\nThis may require elevated privileges.", fileToGet, user, repo, destPath)

		dialog.ShowConfirm("Confirm Get", message, func(confirm bool) {
			if !confirm {
				return
			}
			log.Printf("Getting %s/%s -> %s", user, repo, fileToGet)
			// Here you would execute the download.
			// The polkit part is complex and platform-specific.
			// A common approach is to use `pkexec` to run a helper script/command.
			// For now, we just log it.
			dialog.ShowInformation("Get", "Download would start here.\n(Sudo/Polkit prompt would appear if needed)", w)
		}, w)
	}

	// --- Main Content Area ---
	var searchResults []map[string]string
	resultsList := widget.NewList(
		func() int {
			return len(searchResults)
		},
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				widget.NewButton("Get", nil), // right
				container.NewVBox(
					widget.NewLabel("template/repo"),
					widget.NewLabel("description"),
				),
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

			border := o.(*fyne.Container)
			getBtn := border.Objects[0].(*widget.Button)
			contentBox := border.Objects[1].(*fyne.Container)
			contentBox.Objects[0].(*widget.Label).SetText(repoPath)
			contentBox.Objects[1].(*widget.Label).SetText(desc)

			getBtn.OnTapped = func() {
				getFunc(user, repo)
			}
		},
	)

	// Placeholder content
	placeholder := widget.NewLabel("Use the 'Search' menu to find repositories.")
	placeholder.Alignment = fyne.TextAlignCenter
	content := container.NewMax(placeholder, resultsList)

	// --- Search Logic ---
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search for repositories...")
	searchEntry.OnSubmitted = func(query string) {
		if query != "" {
			matches, err := ftrClient.SearchRepos(query)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			searchResults = matches
			if len(searchResults) > 0 {
				placeholder.Hide()
			} else {
				placeholder.SetText("No matches found.")
				placeholder.Show()
			}
			resultsList.Refresh()
		}
	}

	// --- Menu Bar ---
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("Quit", func() { a.Quit() }),
		),
		fyne.NewMenu("Actions",
			fyne.NewMenuItem("Get", func() { log.Println("Tapped Get") }),
			fyne.NewMenuItem("Sync", func() { log.Println("Tapped Sync") }),
		),
	)
	w.SetMainMenu(mainMenu)

	// --- Custom Title Bar with Buttons ---
	title := widget.NewLabel(appName)
	title.TextStyle.Bold = true

	// minimizeBtn := widget.NewButtonWithIcon("", theme.WindowMinimizeIcon(), func() {
	// 	w.
	// })
	closeBtn := widget.NewButtonWithIcon("", theme.WindowCloseIcon(), func() {
		w.Close()
	})

	titleBar := container.NewBorder(
		nil, nil, // top, bottom
		nil,                         // left
		container.NewHBox(closeBtn), // right
		container.New(
			layout.NewCenterLayout(),
			title,
		),
	)

	// Custom drag handler for the title bar
	dragArea := canvas.NewRectangle(color.Transparent)
	dragArea.SetMinSize(titleBar.MinSize())
	draggableTitleBar := container.NewStack(titleBar, dragArea)

	// --- Window Layout ---
	mainLayout := container.NewBorder(
		container.NewVBox(draggableTitleBar, searchEntry, widget.NewSeparator()),
		nil, nil, nil, // bottom, left, right
		content,
	)

	// Set up the window
	w.SetMaster()
	w.SetPadded(false) // Remove internal padding
	w.SetContent(mainLayout)
	w.Resize(fyne.NewSize(appWidth, appHeight))
	w.CenterOnScreen()
	w.SetFixedSize(false) // Allow resizing

	// Run the app
	w.ShowAndRun()
}
