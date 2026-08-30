// Package gotool is the `go` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// The package is gotool and the directory matches it, because `go` is a
// keyword and cannot name a package.
package gotool

import (
	"context"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "go"

func defaults() shell.Options {
	return shell.Options{Env: shell.Inherit().Blank("CLAUDECODE")}
}

// Run runs the go tool and hands back the result. A non-zero exit is
// Result.Code: every gate treats one as its verdict rather than an error.
func Run(args ...string) (shell.Result, error) {
	return RunWith(defaults(), args...)
}

// RunWith runs the go tool under a caller's options.
func RunWith(opt shell.Options, args ...string) (shell.Result, error) {
	return shell.Run(context.Background(), append([]string{Bin}, args...), opt)
}

// Output runs the go tool and returns its trimmed stdout, reporting a
// non-zero exit as an error.
func Output(args ...string) (string, error) {
	result, err := Run(args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("go %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// Build compiles pkg into binPath. dir is the module to build from, passed as
// -C rather than a working directory so the caller's own cwd is untouched.
func Build(opt shell.Options, dir, binPath, pkg string) error {
	result, err := RunWith(opt, "build", "-C", dir, "-o", binPath, pkg)
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("go build %s: %w", pkg, result.Err())
	}

	return nil
}
