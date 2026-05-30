// Package dto holds passive data transfer objects mirroring the
// third-party JSON wire formats consumed by the testrunner package
// (`go test -json`, `npx playwright test --reporter=json`,
// `npx jest --json`). The field casing matches each tool's emitted
// output, not the project's snake_case convention, so the linter
// excludes this directory from tagliatelle.
package dto

// GoTestEvent mirrors the fields of `go test -json` events the
// runner cares about. Other fields are ignored.
type GoTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}
