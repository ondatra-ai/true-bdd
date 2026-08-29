package shell

import (
	"os"
	"slices"
	"strings"
)

// Env builds the child's environment; the zero value inherits the parent's.
// Blank leaves a key present and empty, Strip removes it: claudecli/run.go:71-73
// strips CLAUDECODE so a nested `claude -p` looks unlaunched-from-a-session.
type Env struct {
	blank []string
	strip []string
	set   []string
	exact []string
}

// Inherit is the parent environment, unchanged. It is the zero value, named
// so a call site can say so.
func Inherit() Env { return Env{} }

// Blank sets each key to empty, leaving it present.
func (e Env) Blank(keys ...string) Env {
	e.blank = append(append([]string(nil), e.blank...), keys...)

	return e
}

// Strip removes each key entirely.
func (e Env) Strip(keys ...string) Env {
	e.strip = append(append([]string(nil), e.strip...), keys...)

	return e
}

// Exact is the whole environment, ignoring the parent's. adapters/ai builds a
// complete one for the agent CLIs and hands it over; everything else derives.
func (e Env) Exact(entries []string) Env {
	e.exact = entries

	return e
}

// Set appends "KEY=value" entries, last wins as os/exec resolves duplicates.
func (e Env) Set(entries ...string) Env {
	e.set = append(append([]string(nil), e.set...), entries...)

	return e
}

// build resolves the child's environment, or nil to inherit the parent's
// untouched — which is what os/exec reads a nil Env as.
func (e Env) build() []string {
	if e.exact != nil {
		return e.exact
	}

	if len(e.blank) == 0 && len(e.strip) == 0 && len(e.set) == 0 {
		return nil
	}

	parent := os.Environ()
	out := make([]string, 0, len(parent)+len(e.blank)+len(e.set))

	for _, entry := range parent {
		name, _, _ := strings.Cut(entry, "=")

		switch {
		case slices.Contains(e.strip, name), slices.Contains(e.blank, name):
			continue
		default:
			out = append(out, entry)
		}
	}

	for _, key := range e.blank {
		out = append(out, key+"=")
	}

	return append(out, e.set...)
}
