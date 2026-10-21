// Package checks holds the engine's built-in checks: the code a `D:`
// Prompt names to answer a Cell without a model turn (ADR 0015). Each
// command walks one Item type and offers the Set of checks that read it;
// a new check is one more entry in its command's Set.
package checks

import (
	"slices"
	"strings"
)

// Verdict is a check's answer in the shape a model's takes: pass or
// fail, plus one context line per finding.
type Verdict struct {
	Pass    bool
	Context []string
}

// Input is what a Cell hands its check beside the Item: the Prompt's
// `with:` parameters, and each document it cites by key, resolved to a path.
type Input struct {
	SubjectID string
	With      map[string]any
	Docs      map[string]string
}

// Check is one built-in check over Items of type I. Params lists the
// `with:` keys it reads; a Prompt passing any other is refused at startup.
type Check[I any] struct {
	Params []string
	Run    func(item I, input Input) (Verdict, error)
}

// Set is the built-in checks a command's `D:` Prompts may name, by name.
type Set[I any] map[string]Check[I]

// Names lists the set's check names in a stable order, for the refusal
// that names what a Prompt could have asked for.
func (s Set[I]) Names() []string {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// Describe renders Names as one phrase: "none" for an empty set.
func (s Set[I]) Describe() string {
	if len(s) == 0 {
		return "none"
	}

	return strings.Join(s.Names(), ", ")
}
