package screen

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ProgressReader is an io.Reader that also updates a progress bar in the console.
type ProgressReader struct {
	R       io.Reader
	Total   int64
	Current int64
	Start   time.Time
}

// Read reads from the underlying reader and updates the progress.
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.R.Read(p)
	pr.Current += int64(n)

	// Calculate percentage
	var percentage float64
	if pr.Total > 0 {
		percentage = float64(pr.Current) / float64(pr.Total) * 100
	} else {
		percentage = 0.0 // Handle case where total is unknown or zero
	}

	// Calculate elapsed time and speed
	elapsed := time.Since(pr.Start).Seconds()
	var speed float64 // bytes per second
	if elapsed > 0 {
		speed = float64(pr.Current) / elapsed
	} else {
		speed = 0.0
	}

	// Convert speed to human-readable format
	var speedStr string
	if speed < 1024 {
		speedStr = fmt.Sprintf("%.0f B/s", speed)
	} else if speed < 1024*1024 {
		speedStr = fmt.Sprintf("%.1f KiB/s", speed/1024)
	} else if speed < 1024*1024*1024 {
		speedStr = fmt.Sprintf("%.1f MiB/s", speed/(1024*1024))
	} else {
		speedStr = fmt.Sprintf("%.1f GiB/s", speed/(1024*1024*1024))
	}

	// Print progress to console (using carriage return to overwrite the line)
	fmt.Fprintf(os.Stderr, "\rProgress: %.1f%% (%s / %s) %s",
		percentage,
		formatBytes(pr.Current),
		formatBytes(pr.Total),
		speedStr,
	)

	// If finished, print a newline to move to the next line
	if err == io.EOF {
		fmt.Fprintln(os.Stderr)
	}
	return n, err
}

// ClearProgressBar clears the last printed progress line.
func ClearProgressBar() {
	// Erase the current line and move cursor to the beginning
	fmt.Fprintf(os.Stderr, "\r%s\r", string(make([]byte, 80, 80)))
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
