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
	// ImportPath names the package a `build-output` event's compiler
	// line belongs to; such events carry no Package.
	ImportPath string `json:"ImportPath"`
	// FailedBuild, on a package's `fail` event, is the ImportPath of the
	// build that failed and kept its tests from running.
	FailedBuild string `json:"FailedBuild"`
}
