package template

import (
	"encoding/json"
	"strings"
	"text/template"
)

var Funcs = template.FuncMap{
	"contains":  strings.Contains,
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
	"toJSON": func(v interface{}) string {
		data, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(data)
	},
	"trimSuffix": strings.TrimSuffix,
	"trimPrefix": strings.TrimPrefix,
}
