package spec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// bashBin parses a host-declared command string. This is the only place in the
// tree that names an interpreter: `npm ci && npx playwright install` has no
// argv form, so the string itself is the contract and something must read it.
const bashBin = "bash"

// Phase is an ordered run of host-declared command strings — a fixture's
// `prep:` before it and `teardown:` after — sharing one budget.
type Phase struct {
	// Name labels a failure: "prep" or "teardown".
	Name string
	// Dir is where every command in the phase runs.
	Dir string
	// Env is what they run under; the zero value inherits the parent's.
	Env shell.Env
	// Budget bounds the WHOLE phase, so each command gets what is LEFT of it.
	// A cold npm install plus a browser download is minutes. Zero: unbounded.
	Budget time.Duration
	// Output opens the sink for the command at index and returns its flush,
	// which is how a caller tees one command to one log. Nil captures.
	Output func(index int) (shell.Sink, func())
}

// Run runs each command in order and stops at the first failure, named by its
// index and text. A blank entry is skipped, so a stray newline in the YAML
// list is not a command.
func (p Phase) Run(commands []string) error {
	failures := p.execute(commands, true)
	if len(failures) > 0 {
		return failures[0]
	}

	return nil
}

// RunAll runs every command whether or not an earlier one failed, and returns
// what each failure was — a teardown must still fire for the commands that
// follow the broken one.
func (p Phase) RunAll(commands []string) []error {
	return p.execute(commands, false)
}

// execute is the loop both entry points share; stop is what tells them apart.
func (p Phase) execute(commands []string, stop bool) []error {
	var (
		deadline time.Time
		failures []error
	)

	if p.Budget > 0 {
		deadline = time.Now().Add(p.Budget)
	}

	for index, raw := range commands {
		command := strings.TrimSpace(raw)
		if command == "" {
			continue
		}

		err := p.one(index, command, deadline)
		if err == nil {
			continue
		}

		failures = append(failures, err)

		if stop {
			return failures
		}
	}

	return failures
}

// one runs a single command string, flushing its sink whatever happened.
func (p Phase) one(index int, command string, deadline time.Time) error {
	output, flush := p.sink(index)

	opt := shell.Options{Dir: p.Dir, Env: p.Env, Output: output}
	if !deadline.IsZero() {
		opt.Timeout = time.Until(deadline)
	}

	result, err := shell.Run(context.Background(), []string{bashBin, "-c", command}, opt)

	flush()

	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return fmt.Errorf("%s[%d] failed (%q): %w", p.Name, index, command, err)
	}

	return nil
}

// sink is Output for this index, or a capture when a caller wants no log.
func (p Phase) sink(index int) (shell.Sink, func()) {
	if p.Output == nil {
		return shell.Capture(), func() {}
	}

	return p.Output(index)
}
