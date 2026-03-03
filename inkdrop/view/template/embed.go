package template

import (
	"embed"
	"io/fs"
	"path"
	"text/template"
)

//go:embed themes/*/*.html
var filesystem embed.FS

func Parse(files ...string) *template.Template {
	if len(files) == 0 {
		panic("Parse requires at least one file")
	}
	name := path.Base(files[0])
	t := template.New(name).Funcs(Funcs)
	res := template.Must(t.ParseFS(filesystem, files...))
	return res
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

	if len(allFiles) == 0 {
		panic("template.ParseBackEndMessage called with no files")
	}
	name := path.Base(allFiles[0])
	t := template.New(name).Funcs(Funcs)
	return template.Must(t.ParseFS(filesystem, allFiles...))
}
