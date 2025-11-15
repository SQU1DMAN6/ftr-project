package screen

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 10 {
		fmt.Println("Warning: Failed to get screen size or screen too small.")
		return 20
	}

	return w
}
