// Package crush is the `crush` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// Three facts about the binary shape this package, all verified against
// v0.88.1 and none documented upstream:
//
//	no permission gate  the caller's hook is the only write gate, and crush
//	                    FAILS OPEN on a hook it cannot run — which is why
//	                    Quote exists and is never applied conditionally.
//	model pins silently a model named in config that crush does not know is
//	                    ignored and global state is used instead, so a model
//	                    is only ever passed as `-m`.
//	embedded shell      a grandchild can hold the stdout pipe past the exit,
//	                    so a turn is spawned as a process group and killed
//	                    as one, with a bounded wait after.
package crush

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "crush"

// GlobalConfigVar names the directory crush loads its config from. It joins
// the config load chain, so a caller supplies permissions and hooks without
// touching the host's own .crush.json.
const GlobalConfigVar = "CRUSH_GLOBAL_CONFIG"

// waitDelay bounds how long Wait blocks after the exit — see the package doc.
const waitDelay = 10 * time.Second

// Turn is one non-interactive run. Prompt goes to stdin because crush has no
// system-prompt flag, so a system prompt has to survive inside the user's.
type Turn struct {
	// Model is passed as -m, never pinned in config.
	Model string
	// WorkDir is both --cwd and the child's working directory.
	WorkDir string
	// Prompt is piped to stdin.
	Prompt string
	// Env is the COMPLETE environment, not a derivation of the parent's.
	Env []string
}

// Args is the argv this turn spawns, exported so a caller can assert on it.
func (t Turn) Args() []string {
	args := []string{Bin, "run", "--quiet"}

	if t.Model != "" {
		args = append(args, "-m", t.Model)
	}

	if t.WorkDir != "" {
		args = append(args, "--cwd", t.WorkDir)
	}

	return args
}

// Run runs the turn and returns crush's combined stdout and stderr, which is
// returned even when the turn failed: it is the transcript a caller archives.
func (t Turn) Run() (string, error) {
	result, err := shell.Run(context.Background(), t.Args(), shell.Options{
		Dir:       t.WorkDir,
		Env:       shell.Env{}.Exact(t.Env),
		Stdin:     strings.NewReader(t.Prompt),
		Output:    shell.Combined(),
		Group:     true,
		WaitDelay: waitDelay,
	})
	if err != nil {
		return result.Stdout, fmt.Errorf("%s: %w", Bin, err)
	}

	if result.Code != 0 {
		return result.Stdout, fmt.Errorf("%s: %w", Bin, result.Err())
	}

	return result.Stdout, nil
}

// Quote wraps a value for a hook command, which crush parses as a shell line.
// Single quotes suppress every metacharacter; an embedded single quote closes
// the run, emits an escaped quote and reopens it.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
