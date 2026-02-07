package template

import (
	"embed"
	"text/template"
)

//go:embed themes/*/*.html
//go:embed themes/*/*.css
var filesystem embed.FS

func ParseBackEndLogin(files ...string) *template.Template {
	allFiles := append(
		[]string{
			"themes/layout/baselogin.html",
		},
		files...)

	return template.Must(
		template.New("").Funcs(Funcs).ParseFS(filesystem, allFiles...))
}
