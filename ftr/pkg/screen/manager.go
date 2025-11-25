package screen

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// progressManager renders a compact block of per-transfer progress lines.
// It is safe for concurrent use from multiple goroutines.
type progressManager struct {
	mu           sync.Mutex
	order        []string
	items        map[string]*progressState
	lastRendered int
	finished     map[string]string // Stores the final rendered line for a finished item
}

type progressState struct {
	Label   string
	Current int64
	Total   int64
	Start   time.Time
}

var manager = &progressManager{
	items:    make(map[string]*progressState),
	finished: make(map[string]string),
}

// Update a named progress entry (creates it if necessary) and re-render the
// whole progress block.
func UpdateProgress(label string, current, total int64, start time.Time) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	st, ok := manager.items[label]
	if !ok {
		st = &progressState{Label: label, Start: start}
		manager.items[label] = st
		manager.order = append(manager.order, label)
	}
	st.Current = current
	st.Total = total
	// Re-render block
	manager.renderLocked()
}

// RemoveProgress removes an entry and re-renders the block.
func RemoveProgress(label string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, ok := manager.items[label]; !ok {
		return
	}
	delete(manager.items, label)
	// rebuild order
	newOrder := make([]string, 0, len(manager.order))
	for _, v := range manager.order {
		if v != label {
			newOrder = append(newOrder, v)
		}
	}
	manager.order = newOrder
	manager.renderLocked()
}

// FinishProgress marks a progress entry as complete, renders it one last time,
// and stores its final state.
func FinishProgress(label string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if st, ok := manager.items[label]; ok {
		st.Current = st.Total
		finalLine := renderProgressLine(st.Label, st.Current, st.Total, st.Start)
		manager.finished[label] = finalLine
		delete(manager.items, label)
	}
}

// Clear clears the whole progress block from the terminal.
func ClearAllProgress() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	// Move cursor up to the start of the last rendered block, clear, and reset
	if manager.lastRendered > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dA", manager.lastRendered)
	}
	fmt.Fprintf(os.Stdout, "\r\033[J")
	manager.order = nil
	manager.items = make(map[string]*progressState)
	manager.finished = make(map[string]string)
	manager.lastRendered = 0
}

func (pm *progressManager) renderLocked() {
	// Move cursor up to overwrite the previous block, clear from cursor to end,
	// then render each progress line on its own row.
	if pm.lastRendered > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dA", pm.lastRendered)
	}
	fmt.Fprintf(os.Stdout, "\r\033[J")
	for _, label := range pm.order {
		var line string
		if finalLine, ok := pm.finished[label]; ok {
			line = finalLine
		} else if st, ok := pm.items[label]; ok {
			line = renderProgressLine(st.Label, st.Current, st.Total, st.Start)
		}
		fmt.Fprintln(os.Stdout, line)
	}
	// Remember how many lines we printed so the next render can overwrite them
	pm.lastRendered = len(pm.order)
}

// renderProgressLine returns a single-line representation similar to the
// previous RenderProgress output, but as a string.
func renderProgressLine(label string, current, total int64, start time.Time) string {
	// reuse logic from progress.go's RenderProgress but return string
	screenwidth := termWidth()

	value := 0.0
	if total > 0 {
		value = float64(current) / float64(total)
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
	}

	// Calculate bar width based on label and other text elements
	barwidth := screenwidth - (len(label) + len(" [] 100% 123.4s elapsed"))
	if barwidth < 10 {
		barwidth = 10
	}

	filled := int(float64(barwidth) * value)

	// Build uncolored bar
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

	// Colorize: '#' and '>' => cyan; '-' => default
	const (
		colorCyan  = "\x1b[36m"
		colorReset = "\x1b[0m"
	)

	var b strings.Builder
	for _, r := range barRunes {
		switch r {
		case '#', '>':
			b.WriteString(colorCyan)
			b.WriteRune(r)
			b.WriteString(colorReset)
		default:
			b.WriteRune(r)
		}
	}
	coloredBar := b.String()

	elapsed := time.Since(start).Seconds()
	elapsed = roundToDecimal(elapsed, 1)

	pct := value * 100
	return fmt.Sprintf("%s [%s] %3.0f%% %.1fs elapsed", label, coloredBar, pct, elapsed)
}
