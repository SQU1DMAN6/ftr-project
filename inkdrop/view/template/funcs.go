package template

import (
	"strings"
	"text/template"
)

var Funcs = template.FuncMap{
	"contains":  strings.Contains,
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
}
