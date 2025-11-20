package screen

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

type ProgressReader struct {
	R       io.Reader
	Total   int64
	Current int64
	Start   time.Time
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.R.Read(p)
	pr.Current += int64(n)
	RenderProgress("   ", pr.Current, pr.Total, pr.Start)
	return n, err
}

func RenderProgress(prefix string, current, total int64, start time.Time) {
	screenwidth := termWidth()

	value := float64(current) / float64(total) // Float value from 0 to 1
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	barwidth := screenwidth - 25
	if barwidth < 10 {
		barwidth = 10
	}

	filled := int(float64(barwidth) * value)

	// Build an uncolored rune bar for width calculation
	barRunes := make([]rune, barwidth)
	for i := 0; i < filled && i < barwidth; i++ {
		barRunes[i] = '#'
	}
	if filled < barwidth {
		barRunes[filled] = '>'
		for i := filled + 1; i < barwidth; i++ {
			barRunes[i] = '-'
		}
	}

	// Colorize the characters for visual appeal. Use ANSI escape codes.
	// '#' and '>' => cyan (36); '-' => deep blue (34)
	const (
		colorCyan  = "\x1b[36m"
		colorBlue  = "\x1b[34m"
		colorReset = "\x1b[0m"
	)

	var b strings.Builder
	// Build colored bar; each rune becomes a colored string
	for _, r := range barRunes {
		switch r {
		case '#', '>':
			b.WriteString(colorCyan)
			b.WriteRune(r)
			b.WriteString(colorReset)
		case '-':
			b.WriteString(colorBlue)
			b.WriteRune(r)
			b.WriteString(colorReset)
		default:
			b.WriteRune(r)
		}
	}
	coloredBar := b.String()

	elapsed := time.Since(start).Seconds()
	elapsed = roundToDecimal(elapsed, 1)

	// Use rune width for printing alignment
	pct := value * 100
	// Compose final line
	fmt.Fprintf(os.Stdout, "\r%s [%s] %3.0f%% %.1fs elapsed\r", prefix, coloredBar, pct, elapsed)
}

func ClearProgressBar() {
	width := termWidth()
	fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", width))
	fmt.Println()
}

func roundToDecimal(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
