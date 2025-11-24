package screen

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 10 {
		fmt.Fprintln(os.Stderr, "Warning: Failed to get screen size or screen too small.")
		return 80
	}

	return w
}
