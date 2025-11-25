package screen

import (
	"fmt"
	"io"
	"math"
	"time"
)

// ProgressReader is an io.Reader that tracks progress with throttled rendering
type ProgressReader struct {
	R          io.Reader
	Total      int64
	Current    int64
	Start      time.Time
	Label      string
	lastRender time.Time
}

// NewProgressReader creates a new progress reader with the given reader, total size, and label.
func NewProgressReader(r io.Reader, total int64, label string) *ProgressReader {
	return &ProgressReader{
		R:          r,
		Total:      total,
		Current:    0,
		Start:      time.Now(),
		Label:      label,
		lastRender: time.Now(),
	}
}

// Read reads from the underlying reader and updates progress with 0.1s throttling
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.R.Read(p)
	pr.Current += int64(n)

	// If we've reached EOF or completed the total, force a render so the bar
	// reaches 100% immediately. Otherwise throttle renders to 0.1s intervals.
	if err == io.EOF || (pr.Total > 0 && pr.Current >= pr.Total) {
		UpdateProgress(pr.Label, pr.Current, pr.Total, pr.Start)
		pr.lastRender = time.Now()
	} else {
		if time.Since(pr.lastRender) > 100*time.Millisecond {
			UpdateProgress(pr.Label, pr.Current, pr.Total, pr.Start)
			pr.lastRender = time.Now()
		}
	}
	return n, err
}

// Finish renders final state and clears the progress bar
func (pr *ProgressReader) Finish() {
	// Ensure final render at 100% (or current state), then emit a newline so
	// the completed bar remains visible for the user rather than being
	// immediately cleared.
	// Final update; leave the final bar in the managed block so callers can
	// clear the block when they print status.
	UpdateProgress(pr.Label, pr.Current, pr.Total, pr.Start)
}

// progressReadCloser wraps an underlying ReadCloser and a ProgressReader so
// that closing the wrapper will finish progress rendering and close the
// underlying reader.
type progressReadCloser struct {
	pr *ProgressReader
	rc io.ReadCloser
}

func (p *progressReadCloser) Read(b []byte) (int, error) {
	return p.pr.Read(b)
}

func (p *progressReadCloser) Close() error {
	// Finish render, then close underlying reader
	p.pr.Finish()
	return p.rc.Close()
}

// WrapReadCloserWithProgress returns an io.ReadCloser that renders progress
// while reading from the provided ReadCloser. The returned closer will call
// Finish() on the progress when Close() is invoked.
func WrapReadCloserWithProgress(rc io.ReadCloser, total int64, label string) io.ReadCloser {
	pr := NewProgressReader(rc, total, label)
	return &progressReadCloser{pr: pr, rc: rc}
}

// RenderProgress renders a single-line progress bar with the format:
// \r<label> [###>----] X% X.Xs elapsed
// RenderProgress is now managed centrally by manager.UpdateProgress
func RenderProgress(label string, current, total int64, start time.Time) {
	UpdateProgress(label, current, total, start)
}

// ClearProgressBar clears the last printed progress line
func ClearProgressBar() {
	// Clear the entire managed block for a tidy single-line message print
	ClearAllProgress()
}

// roundToDecimal rounds a float to a specific decimal precision
func roundToDecimal(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// SuggestLoginError wraps errors that may indicate login is needed
func SuggestLoginError(err error) error {
	return fmt.Errorf("%w (hint: try 'ftr login' if you're not authenticated)", err)
}
