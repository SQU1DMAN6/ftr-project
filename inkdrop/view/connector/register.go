package viewBackend

import (
	"inkdrop/view/template"
	"io"
)

func RegisterMain(w io.Writer, p FrontEndParams) error {
	return template.RegisterMain.Execute(w, p)
}

func RenderSuccessfulRegister(w io.Writer, p FrontEndParams) error {
	return template.RenderSuccessfulRegister.ExecuteTemplate(w, "message.html", p)
}
