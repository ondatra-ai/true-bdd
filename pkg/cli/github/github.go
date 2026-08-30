// Package github is the `gh` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// Three shapes cover every call in the tree: `pr <verb>`, `api` (including
// `api graphql`), and `run list`. JSON exists because scripts/merge decoded
// gh's answer at 20-odd call sites and hand-wrote the diagnostic each time.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "gh"

// ErrUnparseable reports that gh's answer was not the JSON it was asked for.
var ErrUnparseable = errors.New("gh did not answer with JSON")

func defaults() shell.Options {
	return shell.Options{Env: shell.Inherit().Blank("CLAUDECODE")}
}

// Run runs gh and hands back the result. A non-zero exit is Result.Code.
func Run(args ...string) (shell.Result, error) {
	return RunWith(defaults(), args...)
}

// RunWith runs gh under a caller's options.
func RunWith(opt shell.Options, args ...string) (shell.Result, error) {
	return shell.Run(context.Background(), append([]string{Bin}, args...), opt)
}

// Output runs gh and returns its trimmed stdout, reporting a non-zero exit as
// an error.
func Output(args ...string) (string, error) {
	result, err := Run(args...)
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), result.Err())
	}

	return strings.TrimSpace(result.Stdout), nil
}

// JSON runs gh and decodes its answer into target. An empty answer decodes to
// nothing and is not an error: several gh queries legitimately return none.
func JSON(target any, args ...string) error {
	out, err := Output(args...)
	if err != nil {
		return err
	}

	if out == "" {
		return nil
	}

	err = json.Unmarshal([]byte(out), target)
	if err != nil {
		return fmt.Errorf("%w: gh %s: %w", ErrUnparseable, strings.Join(args, " "), err)
	}

	return nil
}
