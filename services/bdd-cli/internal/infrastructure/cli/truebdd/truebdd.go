// Package truebdd is the engine's own binary spawned as a child of itself for
// its hidden verb, and is one of the three engine wrappers that alone may reach
// pkg/shell from inside the engine.
//
// Self names no path: it resolves `os.Executable()` itself, which leaves a
// caller no way to spawn a binary other than the running one.
package truebdd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// CrushGuardSubcommand is the hidden PreToolUse write gate crush calls back into.
const CrushGuardSubcommand = "crush-guard"

// Binary is which true-bdd a spawn runs: always this process's own executable.
type Binary struct{}

// Self is the running executable — the only binary the engine spawns.
func Self() Binary {
	return Binary{}
}

// Path resolves the binary a spawn runs.
func (Binary) Path() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating this executable: %w", err)
	}

	return self, nil
}

// ProbeCrushGuard runs the guard once under a caller's environment and hands
// back its exit code and reason. crush FAILS OPEN on a hook it cannot run, so what the
// caller does with the code is the whole point of asking.
func (b Binary) ProbeCrushGuard(env shell.Env) (shell.Result, error) {
	path, err := b.Path()
	if err != nil {
		return shell.Result{Code: shell.NotStarted}, err
	}

	result, err := shell.Run(context.Background(), []string{path, CrushGuardSubcommand},
		shell.Options{Stdin: strings.NewReader(""), Env: env, Output: shell.Capture()})
	if err != nil {
		return result, fmt.Errorf("probing %s: %w", CrushGuardSubcommand, err)
	}

	return result, nil
}
