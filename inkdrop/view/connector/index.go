package viewBackend

import (
	"inkdrop/view/template"
	"io"
)

func IndexMain(w io.Writer, p FrontEndParams) error {
	return template.IndexMain.Execute(w, p)
}
