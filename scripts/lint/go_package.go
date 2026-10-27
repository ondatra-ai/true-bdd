package lint

import (
	"io"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/golint"
	"github.com/ondatra-ai/true-bdd/scripts/gomodules"
)

// GoPackage lints every repository module, or the named packages in their owning modules.
// Fixture sentinel modules select no Go lint run.
func GoPackage(out io.Writer, dirs []string, fix bool) error {
	selected := map[string][]string{}

	if len(dirs) == 0 {
		for _, module := range gomodules.Directories() {
			selected[module] = []string{"./..."}
		}
	}

	for _, dir := range dirs {
		module, relative, ok := gomodules.Owner(dir)
		if ok && !slices.Contains(selected[module], "./"+relative) {
			selected[module] = append(selected[module], "./"+relative)
		}
	}

	for _, module := range gomodules.Directories() {
		if packages := selected[module]; len(packages) > 0 {
			err := verdictOf(golint.Lint(out, fix, module, packages...))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// verdictOf turns a linter's exit code into this package's sentinel.
func verdictOf(result cli.Result, err error) error {
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return ErrFailed
	}

	return nil
}
