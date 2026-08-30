// Package truebdd is this repository's own binary spawned as a child of
// itself, and is one of the typed wrappers pkg/shell may be reached through.
//
// Self names no path. Every production caller passed `os.Executable()` anyway
// — resolved once in remote's agent and threaded through three structs to
// arrive as a constant — so resolving it here deletes that plumbing and leaves
// a caller no way to spawn a binary other than the running one.
//
// Built is the exception, and it is one: a test cannot ask the test binary for
// `true-bdd version`, so the two gated-supervisor regressions build the real
// thing. A third caller is a design error, not a precedent.
package truebdd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// The hidden verbs. They live here, with the spawn that writes them, and the
// dispatch that answers them reads them back — services may import pkg, so one
// source of truth is reachable in the only direction the layering allows.
const (
	// SupervisorSubcommand runs the resident gated group-leader launcher.
	SupervisorSubcommand = "remote-supervisor"
	// CrushGuardSubcommand is the PreToolUse write gate crush calls back into.
	CrushGuardSubcommand = "crush-guard"
)

// ReleaseFD is where StartGated puts the release pipe's read end. The child
// reads it as fd 3; nothing else may occupy that descriptor.
const ReleaseFD = 3

// Binary is which true-bdd a spawn runs. The zero value is this process's own
// executable, which is what every production caller wants.
type Binary struct {
	// path is empty for Self. Unexported so a caller cannot name a binary
	// without saying, at the call site, that it is a built one.
	path string
}

// Self is the running executable — the only binary production code spawns.
func Self() Binary {
	return Binary{}
}

// Built is a true-bdd compiled to a path. Callers: the two gated-supervisor
// regressions in services/bdd-cli/internal/app/remote, which need a binary
// that answers `version` and so cannot use the test binary.
func Built(path string) Binary {
	return Binary{path: path}
}

// Path resolves the binary, for the two callers that report it rather than
// spawn it: the crush guard's diagnostic and the config hook command.
func (b Binary) Path() (string, error) {
	if b.path != "" {
		return b.path, nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating this executable: %w", err)
	}

	return self, nil
}

// Child is a spawn of a true-bdd in a process group of its own, with its
// stdio on pipes.
type Child struct {
	// Args is the verb and its arguments — never the binary.
	Args []string
	// Dir is the child's working directory.
	Dir string
	// Env is the COMPLETE environment, not a derivation of the parent's.
	Env []string
	// Lock is a held flock inherited by the child so the host folder stays
	// held if the parent dies. Nil when there is none.
	Lock *os.File
}

// StartGroup starts the child as its own process-group leader and returns
// without waiting: its lifetime belongs to the caller's escalation path, not
// to a context.
func (b Binary) StartGroup(child Child) (*shell.Process, error) {
	options := shell.Options{
		Dir:    child.Dir,
		Env:    shell.Env{}.Exact(child.Env),
		Output: shell.Pipe(),
		Group:  true,
	}

	if child.Lock != nil {
		options.ExtraFiles = []*os.File{child.Lock}
	}

	return b.start(child.Args, options)
}

// StartGated starts the RESIDENT GATED supervisor: a group leader blocked on
// the release pipe until the parent has recorded its identity, so a parent that
// dies first never runs the command. Read end on ReleaseFD, write end stays.
func (b Binary) StartGated(child Child, release *os.File) (*shell.Process, error) {
	return b.start(append([]string{SupervisorSubcommand}, child.Args...), shell.Options{
		Dir:        child.Dir,
		Env:        shell.Env{}.Exact(child.Env),
		Output:     shell.Pipe(),
		Group:      true,
		ExtraFiles: []*os.File{release},
	})
}

// Exec re-runs this binary in the CALLER's process group, on the caller's
// terminal. No Group: the supervisor stays the verifiable group leader, so the
// command it launches must not lead a group of its own.
func (b Binary) Exec(args []string, stdin io.Reader) (*shell.Process, error) {
	return b.start(args, shell.Options{Stdin: stdin, Output: shell.Console()})
}

// ProbeCrushGuard runs the guard once under a caller's environment and hands
// back its exit code. crush FAILS OPEN on a hook it cannot run, so what the
// caller does with the code is the whole point of asking.
func (b Binary) ProbeCrushGuard(env shell.Env) (shell.Result, error) {
	path, err := b.Path()
	if err != nil {
		return shell.Result{Code: shell.NotStarted}, err
	}

	result, err := shell.Run(context.Background(), []string{path, CrushGuardSubcommand},
		shell.Options{Stdin: strings.NewReader(""), Env: env, Output: shell.Discard()})
	if err != nil {
		return result, fmt.Errorf("probing %s: %w", CrushGuardSubcommand, err)
	}

	return result, nil
}

// start resolves the binary and spawns it. context.Background: every child
// here outlives the call, and its end is an explicit kill, not a cancellation.
func (b Binary) start(args []string, options shell.Options) (*shell.Process, error) {
	path, err := b.Path()
	if err != nil {
		return nil, err
	}

	proc, err := shell.Start(context.Background(), append([]string{path}, args...), options)
	if err != nil {
		return nil, fmt.Errorf("starting true-bdd: %w", err)
	}

	return proc, nil
}
