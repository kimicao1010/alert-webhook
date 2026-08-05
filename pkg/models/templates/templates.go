package templates

import (
	_ "embed"
)

//go:embed default.tmpl
var DefaultTmpl string
