// Package gotool is the `go` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// The package is gotool and the directory matches it, because `go` is a
// keyword and cannot name a package. Few operations, because few callers: the
// gate table runs `go test`/`go vet` as argv-as-data through pkg/cli/spec,
// and scripts/enginecoverage builds a covered binary and reads its counters.
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

// BuildCovered compiles pkg into binPath with coverage counters on every
// package coverPkg matches. Each run of the binary with GOCOVERDIR set writes
// its counters there on exit. No VCS stamp: go1.25 fails it in a git worktree.
func BuildCovered(opt shell.Options, dir, binPath, pkg, coverPkg string) error {
	result, err := shell.Run(context.Background(), []string{
		Bin, "build", "-C", dir, "-buildvcs=false", "-cover", "-coverpkg=" + coverPkg, "-o", binPath, pkg,
	}, opt)
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("go build -cover %s: %w", pkg, result.Err())
	}

	return nil
}

// CoverProfile merges every counter file in dataDir into one text profile at
// profilePath, the format `go tool cover` reads.
func CoverProfile(opt shell.Options, dataDir, profilePath string) error {
	result, err := shell.Run(context.Background(), []string{
		Bin, "tool", "covdata", "textfmt", "-i=" + dataDir, "-o=" + profilePath,
	}, opt)
	if err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("go tool covdata textfmt: %w", result.Err())
	}

	return nil
}
