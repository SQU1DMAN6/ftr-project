package gui

import (
	"fmt"
	"inker4win/pkg/registry"
	"inker4win/pkg/updater"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// InstalledView displays FtR-managed installed packages
type InstalledView struct {
	widget.BaseWidget
	reg           *registry.Registry
	theme         *InkerTheme
	list          *widget.List
	packages      []registry.PackageInfo
	mux           sync.RWMutex
	onInstallMore func()
	onRemove      func(pkg *registry.PackageInfo)
	onUpdate      func(pkg *registry.PackageInfo)
	filterText    string
}

// NewInstalledView creates a new installed packages view
func NewInstalledView(reg *registry.Registry, theme *InkerTheme) *InstalledView {
	view := &InstalledView{
		reg:   reg,
		theme: theme,
	}
	view.ExtendBaseWidget(view)
	view.refresh()
	return view
}

// refresh reloads the package list from registry
func (iv *InstalledView) refresh() {
	iv.mux.Lock()
	defer iv.mux.Unlock()

	allPackages := iv.reg.GetAllPackages()

	// Filter to only show FtR-managed packages (those with user/repo source)
	var ftrPackages []registry.PackageInfo
	for _, pkg := range allPackages {
		if isFtrManagedPackage(&pkg) {
			ftrPackages = append(ftrPackages, pkg)
		}
	}

	// Further filter by search text if provided
	if iv.filterText != "" {
		var filtered []registry.PackageInfo
		for _, pkg := range ftrPackages {
			if containsIgnoreCase(pkg.Name, iv.filterText) ||
				containsIgnoreCase(pkg.Description, iv.filterText) ||
				containsIgnoreCase(pkg.Source, iv.filterText) {
				filtered = append(filtered, pkg)
			}
		}
		iv.packages = filtered
	} else {
		iv.packages = ftrPackages
	}

	if iv.list != nil {
		iv.list.Refresh()
	}
}

// SetFilter updates the search filter
func (iv *InstalledView) SetFilter(text string) {
	iv.filterText = text
	iv.refresh()
}

// SetCallbacks sets the action callbacks
func (iv *InstalledView) SetCallbacks(
	onInstallMore func(),
	onRemove func(*registry.PackageInfo),
	onUpdate func(*registry.PackageInfo),
) {
	iv.onInstallMore = onInstallMore
	iv.onRemove = onRemove
	iv.onUpdate = onUpdate
}

// CreateRenderer creates the view renderer
func (iv *InstalledView) CreateRenderer() fyne.WidgetRenderer {
	scheme := iv.theme.GetColorScheme()

	// Header with count and install button
	iv.mux.RLock()
	count := len(iv.packages)
	iv.mux.RUnlock()

	countText := canvas.NewText(
		fmt.Sprintf("%d Installed", count),
		scheme.Text,
	)
	countText.TextSize = 14
	countText.TextStyle.Bold = true

	installBtn := widget.NewButton("Install Package", iv.onInstallMore)

	header := container.NewHBox(
		countText,
		layout.NewSpacer(),
		installBtn,
	)

	// Package list
	iv.list = widget.NewList(
		func() int {
			iv.mux.RLock()
			defer iv.mux.RUnlock()
			return len(iv.packages)
		},
		func() fyne.CanvasObject {
			return NewPackageListItem(iv.theme)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			iv.mux.RLock()
			if id >= len(iv.packages) {
				iv.mux.RUnlock()
				return
			}
			pkg := iv.packages[id]
			iv.mux.RUnlock()

			item := obj.(*PackageListItem)
			item.SetPackage(&pkg, false)
			item.OnRemove = func() {
				if iv.onRemove != nil {
					iv.onRemove(&pkg)
				}
			}
		},
	)

	// Determine content: list or empty state
	iv.mux.RLock()
	isEmpty := len(iv.packages) == 0
	iv.mux.RUnlock()

	var content fyne.CanvasObject = iv.list
	if isEmpty {
		empty := EmptyState(
			"No Installed Packages",
			"Click 'Install Package' to get started",
			iv.theme,
		)
		content = &empty
	}

	mainContent := container.NewVBox(header, content)

	return &installedViewRenderer{
		content: mainContent,
		objects: []fyne.CanvasObject{mainContent},
	}
}

type installedViewRenderer struct {
	content fyne.CanvasObject
	objects []fyne.CanvasObject
}

func (r *installedViewRenderer) Layout(size fyne.Size) {
	if r.content != nil {
		r.content.Resize(size)
	}
}

func (r *installedViewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *installedViewRenderer) Refresh() {}

func (r *installedViewRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *installedViewRenderer) Destroy() {}

// UpdatesView displays packages with available updates
type UpdatesView struct {
	widget.BaseWidget
	reg         *registry.Registry
	checker     *updater.UpdateChecker
	theme       *InkerTheme
	list        *widget.List
	packages    []registry.PackageInfo
	mux         sync.RWMutex
	checking    bool
	onUpdate    func(pkg *registry.PackageInfo)
	onUpdateAll func()
	onRefresh   func()
}

// NewUpdatesView creates a new updates view
func NewUpdatesView(reg *registry.Registry, checker *updater.UpdateChecker, theme *InkerTheme) *UpdatesView {
	view := &UpdatesView{
		reg:     reg,
		checker: checker,
		theme:   theme,
	}
	view.ExtendBaseWidget(view)
	view.refresh()
	return view
}

// refresh reloads the updatable packages
func (uv *UpdatesView) refresh() {
	uv.mux.Lock()
	defer uv.mux.Unlock()

	allUpdatable := uv.reg.GetUpdatablePackages()

	// Filter to only show FtR-managed packages
	var ftrUpdatable []registry.PackageInfo
	for _, pkg := range allUpdatable {
		if isFtrManagedPackage(&pkg) {
			ftrUpdatable = append(ftrUpdatable, pkg)
		}
	}

	uv.packages = ftrUpdatable

	if uv.list != nil {
		uv.list.Refresh()
	}
}

// CheckForUpdates triggers a full update check
func (uv *UpdatesView) CheckForUpdates() {
	uv.mux.Lock()
	if uv.checking {
		uv.mux.Unlock()
		return
	}
	uv.checking = true
	uv.mux.Unlock()

	go func() {
		defer func() {
			uv.mux.Lock()
			uv.checking = false
			uv.mux.Unlock()
		}()

		// Build package map (only for FtR-managed packages)
		allPkgs := uv.reg.GetAllPackages()
		packages := make(map[string]updater.PackageVersion)
		for _, pkg := range allPkgs {
			// Only check updates for FtR-managed packages
			if !isFtrManagedPackage(&pkg) {
				continue
			}
			if pkg.Source != "" {
				packages[pkg.Name] = updater.PackageVersion{
					Version: pkg.Version,
					Source:  pkg.Source,
				}
			}
		}

		// Check for updates
		results, _ := uv.checker.CheckBatchUpdates(packages)

		// Update registry with results
		for pkg, newVer := range results {
			if p, _ := uv.reg.GetPackage(pkg); p != nil {
				p.UpdateAvailable = newVer
				p.LastUpdateChecked = time.Now()
				uv.reg.AddPackage(*p)
			}
		}
		uv.reg.Save()

		// Refresh display
		uv.refresh()
		if uv.onRefresh != nil {
			uv.onRefresh()
		}
	}()
}

// SetCallbacks sets the action callbacks
func (uv *UpdatesView) SetCallbacks(
	onUpdate func(*registry.PackageInfo),
	onUpdateAll func(),
	onRefresh func(),
) {
	uv.onUpdate = onUpdate
	uv.onUpdateAll = onUpdateAll
	uv.onRefresh = onRefresh
}

// CreateRenderer creates the view renderer
func (uv *UpdatesView) CreateRenderer() fyne.WidgetRenderer {
	scheme := uv.theme.GetColorScheme()

	// Header with count and actions
	uv.mux.RLock()
	count := len(uv.packages)
	checking := uv.checking
	uv.mux.RUnlock()

	statusText := "No Updates"
	if checking {
		statusText = "Checking for updates..."
	} else if count > 0 {
		statusText = fmt.Sprintf("%d Update Available", count)
		if count > 1 {
			statusText = fmt.Sprintf("%d Updates Available", count)
		}
	}

	statusLabel := canvas.NewText(statusText, scheme.Text)
	statusLabel.TextSize = 14
	statusLabel.TextStyle.Bold = true

	refreshBtn := widget.NewButton("Check Now", func() {
		uv.CheckForUpdates()
	})

	updateAllBtn := widget.NewButton("Update All", uv.onUpdateAll)

	header := container.NewHBox(
		statusLabel,
		layout.NewSpacer(),
		refreshBtn,
		updateAllBtn,
	)

	// Package list
	uv.list = widget.NewList(
		func() int {
			uv.mux.RLock()
			defer uv.mux.RUnlock()
			return len(uv.packages)
		},
		func() fyne.CanvasObject {
			return NewPackageListItem(uv.theme)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			uv.mux.RLock()
			if id >= len(uv.packages) {
				uv.mux.RUnlock()
				return
			}
			pkg := uv.packages[id]
			uv.mux.RUnlock()

			item := obj.(*PackageListItem)
			item.SetPackage(&pkg, true) // Show as updatable
			item.OnUpdate = func() {
				if uv.onUpdate != nil {
					uv.onUpdate(&pkg)
				}
			}
		},
	)

	// Determine content: list or empty state
	if count == 0 && !checking {
		empty := EmptyState(
			"All Up to Date!",
			"You're running the latest versions",
			uv.theme,
		)
		mainContent := container.NewVBox(header, &empty)
		return &updatesViewRenderer{
			content: mainContent,
			objects: []fyne.CanvasObject{mainContent},
		}
	}

	mainContent := container.NewVBox(header, uv.list)

	return &updatesViewRenderer{
		content: mainContent,
		objects: []fyne.CanvasObject{mainContent},
	}
}

type updatesViewRenderer struct {
	content fyne.CanvasObject
	objects []fyne.CanvasObject
}

func (r *updatesViewRenderer) Layout(size fyne.Size) {
	if r.content != nil {
		r.content.Resize(size)
	}
}

func (r *updatesViewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *updatesViewRenderer) Refresh() {}

func (r *updatesViewRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *updatesViewRenderer) Destroy() {}

// PackageListItem is a single item in the package list
type PackageListItem struct {
	widget.BaseWidget
	pkg      *registry.PackageInfo
	isUpdate bool
	theme    *InkerTheme
	OnRemove func()
	OnUpdate func()
}

// NewPackageListItem creates a new package list item
func NewPackageListItem(theme *InkerTheme) *PackageListItem {
	item := &PackageListItem{
		theme: theme,
	}
	item.ExtendBaseWidget(item)
	return item
}

// SetPackage updates the displayed package
func (pli *PackageListItem) SetPackage(pkg *registry.PackageInfo, isUpdate bool) {
	pli.pkg = pkg
	pli.isUpdate = isUpdate
	pli.Refresh()
}

// CreateRenderer creates the renderer for the list item
func (pli *PackageListItem) CreateRenderer() fyne.WidgetRenderer {
	scheme := pli.theme.GetColorScheme()

	if pli.pkg == nil {
		return &packageListItemRenderer{
			objects: []fyne.CanvasObject{},
		}
	}

	// Package name and version
	nameText := canvas.NewText(pli.pkg.Name, scheme.Text)
	nameText.TextSize = 14
	nameText.TextStyle.Bold = true

	versionText := canvas.NewText(
		fmt.Sprintf("v%s", pli.pkg.Version),
		scheme.TextSecondary,
	)
	versionText.TextSize = 12

	// Source (user/repo)
	sourceText := canvas.NewText(
		pli.pkg.Source,
		scheme.TextSecondary,
	)
	sourceText.TextSize = 11

	// Info section
	info := container.NewVBox(
		container.NewHBox(nameText, layout.NewSpacer(), versionText),
		sourceText,
	)

	// Buttons
	var buttons []fyne.CanvasObject
	if pli.isUpdate && pli.OnUpdate != nil {
		updateBtn := widget.NewButton("Update", pli.OnUpdate)
		buttons = append(buttons, updateBtn)
	}
	if !pli.isUpdate && pli.OnRemove != nil {
		removeBtn := widget.NewButton("Remove", pli.OnRemove)
		buttons = append(buttons, removeBtn)
	}

	var content fyne.CanvasObject
	if len(buttons) > 0 {
		buttonObjects := make([]fyne.CanvasObject, 0, len(buttons)+1)
		buttonObjects = append(buttonObjects, layout.NewSpacer())
		buttonObjects = append(buttonObjects, buttons...)
		buttonContainer := container.NewHBox(buttonObjects...)
		content = container.NewVBox(info, buttonContainer)
	} else {
		content = info
	}

	// Background
	bg := canvas.NewRectangle(scheme.SurfaceHover)

	return &packageListItemRenderer{
		bg:      bg,
		content: content,
		objects: []fyne.CanvasObject{bg, content},
	}
}

type packageListItemRenderer struct {
	bg      *canvas.Rectangle
	content fyne.CanvasObject
	objects []fyne.CanvasObject
}

func (r *packageListItemRenderer) Layout(size fyne.Size) {
	if r.bg != nil {
		r.bg.Resize(size)
	}
	if r.content != nil {
		r.content.Move(fyne.NewPos(12, 8))
		r.content.Resize(fyne.NewSize(size.Width-24, size.Height-16))
	}
}

func (r *packageListItemRenderer) MinSize() fyne.Size {
	return fyne.NewSize(300, 80)
}

func (r *packageListItemRenderer) Refresh() {}

func (r *packageListItemRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *packageListItemRenderer) Destroy() {}

// Helper function for case-insensitive string matching
func containsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// isFtrManagedPackage checks if a package is FtR-managed (has user/repo source)
func isFtrManagedPackage(pkg *registry.PackageInfo) bool {
	if pkg == nil {
		return false
	}

	// FtR-managed packages must have a source in "user/repo" format
	if pkg.Source == "" {
		return false
	}

	// Check if source matches user/repo pattern
	parts := strings.Split(pkg.Source, "/")
	if len(parts) != 2 {
		return false
	}

	user := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])

	// Valid source if both user and repo are non-empty
	return user != "" && repo != ""
}
