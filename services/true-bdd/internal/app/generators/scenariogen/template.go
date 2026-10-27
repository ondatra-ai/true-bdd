package scenariogen

import (
	_ "embed"
	"text/template"
)

// goTestTemplate renders one generated Go test file — embedded, since it must compile, unlike prompt templates.
//
//go:embed go_test.go.tmpl
var goTestTemplate string

// fileTemplate parses the embedded template. Must, not an error return: a
// parse failure here is a bug in this package, not something a host caused.
// Parsed per call, not cached at init: a plan renders a handful of files and the parse is microseconds.
func fileTemplate() *template.Template {
	return template.Must(template.New("go-test").Parse(goTestTemplate))
}
