package viewBackend

import (
	"inkdrop/view/template"
	"io"
)

func LoginMain(w io.Writer, p FrontEndParams) error {
	return template.LoginMain.ExecuteTemplate(w, "baselogin.html", p)
}
