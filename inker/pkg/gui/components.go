package gui

import (
	"fmt"
	"image/color"
	"inker/pkg/registry"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// PackageCard is a reusable component for displaying package information
type PackageCard struct {
	widget.BaseWidget
	Package     *registry.PackageInfo
	OnInstall   func()
	OnUpdate    func()
	OnRemove    func()
	Theme       *InkerTheme
	IsUpdatable bool
}

// NewPackageCard creates a new package card widget
func NewPackageCard(pkg *registry.PackageInfo, theme *InkerTheme) *PackageCard {
	card := &PackageCard{
		Package: pkg,
		Theme:   theme,
	}
	card.ExtendBaseWidget(card)
	return card
}

// CreateRenderer creates the renderer for the package card
func (pc *PackageCard) CreateRenderer() fyne.WidgetRenderer {
	scheme := pc.Theme.GetColorScheme()

	// Title
	title := canvas.NewText(pc.Package.Name, scheme.Text)
	title.TextSize = 16
	title.TextStyle.Bold = true

	// Version
	version := canvas.NewText(fmt.Sprintf("v%s", pc.Package.Version), scheme.TextSecondary)
	version.TextSize = 12

	// Description
	desc := canvas.NewText(pc.Package.Description, scheme.TextSecondary)
	desc.TextSize = 12

	// Status badge
	statusText := "Installed"
	statusColor := scheme.Success
	if pc.IsUpdatable {
		statusText = "Update Available"
		statusColor = scheme.Warning
	}

	statusBg := canvas.NewRectangle(WithAlpha(statusColor, 32))
	statusLabel := canvas.NewText(statusText, statusColor)
	statusLabel.TextSize = 10

	// Buttons
	var buttons []fyne.CanvasObject
	if pc.OnUpdate != nil && pc.IsUpdatable {
		updateBtn := widget.NewButton("Update", pc.OnUpdate)
		buttons = append(buttons, updateBtn)
	}
	if pc.OnRemove != nil {
		removeBtn := widget.NewButton("Remove", pc.OnRemove)
		buttons = append(buttons, removeBtn)
	}

	// Header container
	header := container.NewVBox(
		container.NewHBox(
			container.NewVBox(title, version),
			container.NewVBox(statusBg, statusLabel),
		),
		desc,
	)

	// Button container
	buttonContainer := container.NewHBox(buttons...)

	// Layout
	content := container.NewVBox(header, buttonContainer)

	// Background
	bg := canvas.NewRectangle(scheme.Surface)

	return &packageCardRenderer{
		bg:      bg,
		content: content,
		objects: []fyne.CanvasObject{bg, content},
	}
}

type packageCardRenderer struct {
	bg      *canvas.Rectangle
	content *fyne.Container
	objects []fyne.CanvasObject
}

func (r *packageCardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.content.Move(fyne.NewPos(12, 12))
	r.content.Resize(fyne.NewSize(size.Width-24, size.Height-24))
}

func (r *packageCardRenderer) MinSize() fyne.Size {
	return fyne.NewSize(300, 120)
}

func (r *packageCardRenderer) Refresh() {
	// Refresh colors if theme changed
}

func (r *packageCardRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *packageCardRenderer) Destroy() {}

// ProgressBar is a custom progress bar with percentage text
type ProgressBar struct {
	widget.BaseWidget
	Value   float32 // 0 to 100
	Label   string
	Theme   *InkerTheme
	OnClick func()
}

// NewProgressBar creates a new progress bar
func NewProgressBar(theme *InkerTheme) *ProgressBar {
	pb := &ProgressBar{
		Value: 0,
		Theme: theme,
	}
	pb.ExtendBaseWidget(pb)
	return pb
}

// SetProgress updates the progress value
func (pb *ProgressBar) SetProgress(value float32) {
	if value > 100 {
		value = 100
	} else if value < 0 {
		value = 0
	}
	pb.Value = value
	pb.Refresh()
}

// SetLabel updates the label text
func (pb *ProgressBar) SetLabel(label string) {
	pb.Label = label
	pb.Refresh()
}

// CreateRenderer creates the renderer for the progress bar
func (pb *ProgressBar) CreateRenderer() fyne.WidgetRenderer {
	scheme := pb.Theme.GetColorScheme()

	// Background
	bg := canvas.NewRectangle(scheme.SurfaceHover)

	// Progress fill
	fill := canvas.NewRectangle(scheme.Primary)

	// Text
	text := canvas.NewText(fmt.Sprintf("%.0f%%", pb.Value), scheme.Text)
	text.TextSize = 12
	text.Alignment = fyne.TextAlignCenter

	return &progressBarRenderer{
		bg:   bg,
		fill: fill,
		text: text,
		bar:  pb,
	}
}

type progressBarRenderer struct {
	bg   *canvas.Rectangle
	fill *canvas.Rectangle
	text *canvas.Text
	bar  *ProgressBar
}

func (r *progressBarRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	fillWidth := size.Width * r.bar.Value / 100
	r.fill.Resize(fyne.NewSize(fillWidth, size.Height))
	r.text.Move(fyne.NewPos(size.Width/2-r.text.MinSize().Width/2, size.Height/2-r.text.MinSize().Height/2))
	r.text.TextSize = 12
}

func (r *progressBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 32)
}

func (r *progressBarRenderer) Refresh() {
	r.text.Text = fmt.Sprintf("%.0f%%", r.bar.Value)
	r.text.Color = r.bar.Theme.GetColorScheme().Text
	canvas.Refresh(r.text)
}

func (r *progressBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill, r.text}
}

func (r *progressBarRenderer) Destroy() {}

// StatusBadge displays status with color coding
type StatusBadge struct {
	widget.BaseWidget
	Status string // "success", "warning", "error", "info"
	Label  string
	Theme  *InkerTheme
}

// NewStatusBadge creates a new status badge
func NewStatusBadge(status, label string, theme *InkerTheme) *StatusBadge {
	sb := &StatusBadge{
		Status: status,
		Label:  label,
		Theme:  theme,
	}
	sb.ExtendBaseWidget(sb)
	return sb
}

// CreateRenderer creates the renderer for the status badge
func (sb *StatusBadge) CreateRenderer() fyne.WidgetRenderer {
	scheme := sb.Theme.GetColorScheme()

	// Determine color based on status
	var color color.Color
	switch sb.Status {
	case "success":
		color = scheme.Success
	case "warning":
		color = scheme.Warning
	case "error":
		color = scheme.Error
	default:
		color = scheme.Primary
	}

	// Background
	bg := canvas.NewRectangle(WithAlpha(color, 32))

	// Text
	text := canvas.NewText(sb.Label, color)
	text.TextSize = 11

	return &statusBadgeRenderer{
		bg:   bg,
		text: text,
	}
}

type statusBadgeRenderer struct {
	bg   *canvas.Rectangle
	text *canvas.Text
}

func (r *statusBadgeRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.text.Move(fyne.NewPos(6, 4))
}

func (r *statusBadgeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 24)
}

func (r *statusBadgeRenderer) Refresh() {}

func (r *statusBadgeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.text}
}

func (r *statusBadgeRenderer) Destroy() {}

// Card creates a basic card container with styled background
func Card(content fyne.CanvasObject, theme *InkerTheme) fyne.Container {
	scheme := theme.GetColorScheme()
	bg := canvas.NewRectangle(scheme.Surface)

	return *container.NewStack(
		bg,
		container.NewPadded(container.New(layout.NewPaddedLayout(), content)),
	)
}

// PaddedBox creates a padded container
func PaddedBox(content fyne.CanvasObject, padding float32) fyne.Container {
	return *container.New(
		NewPaddedLayout(padding),
		content,
	)
}

// PaddedLayout implements custom padding
type PaddedLayout struct {
	padding float32
}

// NewPaddedLayout creates a new padded layout
func NewPaddedLayout(padding float32) fyne.Layout {
	return &PaddedLayout{padding: padding}
}

// Layout arranges children with padding
func (pl *PaddedLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		obj.Move(fyne.NewPos(pl.padding, pl.padding))
		obj.Resize(fyne.NewSize(size.Width-pl.padding*2, size.Height-pl.padding*2))
	}
}

// MinSize returns minimum size
func (pl *PaddedLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minSize := fyne.NewSize(0, 0)
	for _, obj := range objects {
		objMin := obj.MinSize()
		if objMin.Width > minSize.Width {
			minSize.Width = objMin.Width
		}
		if objMin.Height > minSize.Height {
			minSize.Height = objMin.Height
		}
	}
	return fyne.NewSize(minSize.Width+pl.padding*2, minSize.Height+pl.padding*2)
}

// Separator creates a visual separator
func Separator(theme *InkerTheme) fyne.Container {
	scheme := theme.GetColorScheme()
	line := canvas.NewRectangle(scheme.Divider)
	line.SetMinSize(fyne.NewSize(0, 1))
	return *container.New(
		layout.NewCenterLayout(),
		line,
	)
}

// EmptyState creates an empty state placeholder
func EmptyState(title, message string, theme *InkerTheme) fyne.Container {
	scheme := theme.GetColorScheme()

	titleText := canvas.NewText(title, scheme.Text)
	titleText.TextSize = 16
	titleText.TextStyle.Bold = true

	messageText := canvas.NewText(message, scheme.TextSecondary)
	messageText.TextSize = 12

	return *container.NewVBox(
		container.NewCenter(titleText),
		container.NewCenter(messageText),
	)
}
