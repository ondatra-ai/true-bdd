// Package gotool is the `go` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// The package is gotool and the directory matches it, because `go` is a
// keyword and cannot name a package. One operation, because one caller: the
// gate table runs `go test`/`go vet` as argv-as-data through pkg/cli/spec.
package gotool

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "go"

// Build compiles pkg into binPath. dir is the module to build from, passed as
// -C rather than a working directory so the caller's own cwd is untouched.
func Build(opt shell.Options, dir, binPath, pkg string) error {
	result, err := shell.Run(context.Background(),
		[]string{Bin, "build", "-C", dir, "-o", binPath, pkg}, opt)
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("go build %s: %w", pkg, result.Err())
	}

	return nil
}
