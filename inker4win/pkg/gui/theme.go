package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ColorScheme defines a complete color palette
type ColorScheme struct {
	Primary       color.Color
	Secondary     color.Color
	Accent        color.Color
	Success       color.Color
	Warning       color.Color
	Error         color.Color
	Background    color.Color
	Surface       color.Color
	SurfaceHover  color.Color
	Text          color.Color
	TextSecondary color.Color
	Border        color.Color
	BorderLight   color.Color
	Divider       color.Color
}

// DarkScheme is the dark theme color palette
var DarkScheme = ColorScheme{
	Primary:       color.RGBA{R: 0x25, G: 0x63, B: 0xEB, A: 0xFF}, // #2563EB
	Secondary:     color.RGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}, // #10B981
	Accent:        color.RGBA{R: 0xF5, G: 0x9E, B: 0x0B, A: 0xFF}, // #F59E0B
	Success:       color.RGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}, // #10B981
	Warning:       color.RGBA{R: 0xF5, G: 0x9E, B: 0x0B, A: 0xFF}, // #F59E0B
	Error:         color.RGBA{R: 0xEF, G: 0x44, B: 0x44, A: 0xFF}, // #EF4444
	Background:    color.RGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // #0F172A
	Surface:       color.RGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}, // #1E293B
	SurfaceHover:  color.RGBA{R: 0x2D, G: 0x3A, B: 0x4D, A: 0xFF}, // Lighter surface
	Text:          color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // #E2E8F0
	TextSecondary: color.RGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}, // #94A3B8
	Border:        color.RGBA{R: 0x33, G: 0x4E, B: 0x6E, A: 0xFF}, // Semi-transparent border
	BorderLight:   color.RGBA{R: 0x47, G: 0x55, B: 0x69, A: 0xFF}, // Lighter border
	Divider:       color.RGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}, // Subtle divider
}

// LightScheme is the light theme color palette
var LightScheme = ColorScheme{
	Primary:       color.RGBA{R: 0x25, G: 0x63, B: 0xEB, A: 0xFF}, // #2563EB
	Secondary:     color.RGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}, // #10B981
	Accent:        color.RGBA{R: 0xF5, G: 0x9E, B: 0x0B, A: 0xFF}, // #F59E0B
	Success:       color.RGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}, // #10B981
	Warning:       color.RGBA{R: 0xF5, G: 0x9E, B: 0x0B, A: 0xFF}, // #F59E0B
	Error:         color.RGBA{R: 0xEF, G: 0x44, B: 0x44, A: 0xFF}, // #EF4444
	Background:    color.RGBA{R: 0xF8, G: 0xFA, B: 0xFC, A: 0xFF}, // #F8FAFC
	Surface:       color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // #FFFFFF
	SurfaceHover:  color.RGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}, // Slightly darker surface
	Text:          color.RGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // #0F172A
	TextSecondary: color.RGBA{R: 0x64, G: 0x74, B: 0x8B, A: 0xFF}, // #64748B
	Border:        color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // #E2E8F0
	BorderLight:   color.RGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}, // #F1F5F9
	Divider:       color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // Subtle divider
}

// InkerTheme implements fyne's Theme interface with custom colors
type InkerTheme struct {
	isDark bool
	scheme ColorScheme
}

// NewTheme creates a new Inker theme
func NewTheme(isDark bool) *InkerTheme {
	scheme := DarkScheme
	if !isDark {
		scheme = LightScheme
	}
	return &InkerTheme{
		isDark: isDark,
		scheme: scheme,
	}
}

// Color implements the fyne Theme interface
func (t *InkerTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.isDark {
		variant = theme.VariantDark
	} else {
		variant = theme.VariantLight
	}

	switch name {
	case theme.ColorNamePrimary:
		return t.scheme.Primary
	case theme.ColorNameFocus:
		return t.scheme.Primary
	case theme.ColorNameSuccess:
		return t.scheme.Success
	case theme.ColorNameWarning:
		return t.scheme.Warning
	case theme.ColorNameError:
		return t.scheme.Error
	case theme.ColorNameForeground:
		return t.scheme.Text
	case theme.ColorNameBackground:
		return t.scheme.Background
	case theme.ColorNamePlaceHolder:
		return t.scheme.TextSecondary
	case theme.ColorNameSelection:
		return color.RGBA{R: 0x25, G: 0x63, B: 0xEB, A: 0x44} // Primary with alpha
	default:
		return theme.ForegroundColor()
	}
}

// Icon returns the icon for the given name
func (t *InkerTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.NewThemedResource(theme.Icon(name))
}

// Font returns the font for the given style
func (t *InkerTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Size returns the size for the given name
func (t *InkerTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameLineSpacing:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 2
	case theme.SizeNameScrollBar:
		return 16
	case theme.SizeNameScrollBarSmall:
		return 8
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// IsDark returns whether this is a dark theme
func (t *InkerTheme) IsDark() bool {
	return t.isDark
}

// SetDark switches the theme mode
func (t *InkerTheme) SetDark(isDark bool) {
	t.isDark = isDark
	if isDark {
		t.scheme = DarkScheme
	} else {
		t.scheme = LightScheme
	}
}

// GetColorScheme returns the current color scheme
func (t *InkerTheme) GetColorScheme() ColorScheme {
	return t.scheme
}

// Helper function to adjust color brightness
func AdjustBrightness(c color.Color, factor float32) color.Color {
	r, g, b, a := c.RGBA()
	r = uint32(float32(r>>8) * float32(factor))
	g = uint32(float32(g>>8) * float32(factor))
	b = uint32(float32(b>>8) * float32(factor))
	return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a >> 8)}
}

// Helper function to blend two colors
func BlendColors(c1, c2 color.Color, ratio float32) color.Color {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	r := uint8(float32(r1>>8)*(1-ratio) + float32(r2>>8)*ratio)
	g := uint8(float32(g1>>8)*(1-ratio) + float32(g2>>8)*ratio)
	b := uint8(float32(b1>>8)*(1-ratio) + float32(b2>>8)*ratio)
	a := uint8(float32(a1>>8)*(1-ratio) + float32(a2>>8)*ratio)

	return color.RGBA{r, g, b, a}
}

// Helper function to create semi-transparent color
func WithAlpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), alpha}
}
