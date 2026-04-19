package viewBackend

import (
	"fmt"
	"inkdrop/view/template"
	"io"
)

func LiveEditTextFile(w io.Writer, p FrontEndParams) error {
	err := template.LiveEditTextFile.Execute(w, p)
	if err != nil {
		fmt.Printf("[template] LiveEditTextFile error: %v\n", err)
	}
	return err
}
