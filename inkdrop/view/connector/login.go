package viewBackend

import (
	"fmt"
	"inkdrop/view/template"
	"io"
)

func LoginMain(w io.Writer, p FrontEndParams) error {
	err := template.LoginMain.Execute(w, p)
	if err != nil {
		fmt.Printf("[template] LoginMain error: %v\n", err)
	}
	return err
}
