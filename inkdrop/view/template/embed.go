package template

import (
	"embed"
	"io/fs"
	"text/template"
)

//go:embed themes/*/*.html
var filesystem embed.FS

func Parse(files ...string) *template.Template {
	return template.Must(
		template.ParseFS(filesystem, files...))
}

func GetAssetsFS() fs.FS {
	sub, err := fs.Sub(filesystem, "themes/assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func ParseBackEndMessage(files ...string) *template.Template {
	allFiles := append(
		[]string{
			"themes/base/message.html",
		},
		files...)

	return template.Must(
		template.New("").Funcs(Funcs).ParseFS(filesystem, allFiles...))
}
