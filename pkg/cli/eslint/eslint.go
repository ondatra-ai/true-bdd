// Package eslint is the `eslint` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// Two facts about this binary shape the package. It is not on PATH — it is a
// devDependency of the frontend, so the binary lives under that tree's
// node_modules — and it finds `eslint.config.mjs` by walking up from its
// working directory, so it must be RUN from that tree with paths relative to
// it. A caller names a repository-relative file and gets both for free.
package eslint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Root is the tree eslint is installed in and configured for; Bin is where
// `npm install` puts it.
const (
	Root = "services/bdd-web"
	Bin  = "node_modules/.bin/eslint"
)

// Install is the line that gets it.
const Install = "npm --prefix " + Root + " install"

// ErrNotInstalled reports that the frontend's devDependencies are absent.
var ErrNotInstalled = errors.New("eslint is not installed")

// Available reports whether the frontend's eslint is installed, naming the
// install line rather than leaving a caller with a mystery spawn error.
func Available() error {
	_, err := os.Stat(filepath.Join(Root, Bin))
	if err != nil {
		return fmt.Errorf("%w — run: %s", ErrNotInstalled, Install)
	}

	return nil
}

// Owns reports whether a repository-relative path is one eslint is configured
// for. Anything outside Root has no config to find and is not this gate's.
func Owns(path string) bool {
	return strings.HasPrefix(filepath.Clean(path), Root+string(filepath.Separator))
}

// Lint checks the named repository-relative files, rewriting what it can when
// fix is set. A non-zero Result.Code is a finding, not an error.
func Lint(out io.Writer, fix bool, files ...string) (shell.Result, error) {
	argv := []string{Bin}
	if fix {
		argv = append(argv, "--fix")
	}

	for _, file := range files {
		relative, err := filepath.Rel(Root, filepath.Clean(file))
		if err != nil {
			return shell.Result{}, fmt.Errorf("%s is not under %s: %w", file, Root, err)
		}

		argv = append(argv, relative)
	}

	return shell.Run(context.Background(), argv,
		shell.Options{Dir: Root, Output: shell.To(out)})
}
