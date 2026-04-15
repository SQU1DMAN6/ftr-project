package viewBackend

import (
	"fmt"
	"inkdrop/view/template"
	"io"
)

func DocumentEditFile(w io.Writer, p FrontEndParams) error {
	err := template.DocumentEditFile.Execute(w, p)
	if err != nil {
		fmt.Printf("[template] DocumentEditFile error: %v\n", err)
	}
	return err
}
