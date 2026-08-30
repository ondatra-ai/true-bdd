package lint

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/golint"
)

// GoPackage runs golangci-lint over the named directories, or the whole module
// when none are named. The package is the floor: `golangci-lint run <file>.go`
// typechecks that file ALONE — all 17 package-mates came back "undefined".
func GoPackage(out io.Writer, dirs []string, fix bool) error {
	args := []string{"run"}
	if fix {
		args = append(args, "--fix")
	}

	scoped := analysable(dirs)
	if len(dirs) > 0 && len(scoped) == 0 {
		return nil
	}

	return verdictOf(golint.Run(out, append(args, scoped...)...))
}

// analysable drops the directories a sentinel go.mod fences out of the root
// module: golangci-lint asked to lint inside one exits 5 with "no go files to
// analyze", so such a directory selects no Go gate at all.
func analysable(dirs []string) []string {
	var kept []string

	for _, dir := range dirs {
		clean := strings.TrimPrefix(filepath.Clean(dir), "./")
		if fenced(clean) {
			continue
		}

		target := "./" + clean
		if !slices.Contains(kept, target) {
			kept = append(kept, target)
		}
	}

	return kept
}

// fenced reports whether a sentinel go.mod between dir and the root takes it
// out of this module.
func fenced(dir string) bool {
	for walk := dir; walk != "." && walk != string(filepath.Separator); walk = filepath.Dir(walk) {
		_, err := os.Stat(filepath.Join(walk, "go.mod"))
		if err == nil {
			return true
		}
	}

	return false
}

// verdictOf turns a linter's exit code into this package's sentinel: the tool
// has already printed its findings, so nothing more is said about them.
func verdictOf(result cli.Result, err error) error {
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return ErrFailed
	}

	return nil
}
