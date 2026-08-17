package scenariogen

import (
	_ "embed"
	"text/template"
)

// goTestTemplate renders one generated Go test file.
//
// Embedded in the package rather than shipped under templates/, and the
// distinction is not filing. Those are PROMPT templates: text a host
// tunes to change what a model is asked. This one has to compile against
// bddgo's API, so a host that edited it would produce a repository that
// does not build — and saying so structurally is better than saying so
// in a comment nobody reads until it is too late.
//
//go:embed go_test.go.tmpl
var goTestTemplate string

// fileTemplate parses the embedded template. Must, not an error return:
// a template that will not parse is a bug in this package rather than
// anything a host could have caused, and there is no input to blame.
//
// Parsed per call rather than once at init, because a plan renders a
// handful of files and the parse is microseconds — not worth a global
// whose initialisation order has to be reasoned about.
func fileTemplate() *template.Template {
	return template.Must(template.New("go-test").Parse(goTestTemplate))
}
