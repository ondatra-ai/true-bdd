// Package alint is the `alint` command line, one of the typed wrappers
// pkg/shell may be reached through: Check and Fix start the binary and decode
// what it answers.
//
// It is the outbound half only. Being spawned BY alint — the child of a
// `kind: command` rule — is pkg/alint, because that is a protocol this
// repository is handed rather than one it writes.
//
// Two facts about the binary shape this package, both verified against 0.15.2
// and neither documented upstream:
//
//	fix -f agent    rejected. `fix` accepts human, json and markdown only,
//	                so both subcommands are read as `-f json` here.
//	stdout is JSON  the walker's warnings go to stderr, so the streams are
//	                captured apart and only stdout is decoded.
package alint

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "alint"

// ScopeVar names the manifest a run is restricted to. Only Fix sets it: the
// scoped rules in .alint.yml carry --fix in their argv, so a checking run that
// carried a scope would rewrite the files it was asked to report on.
const ScopeVar = "TRUEBDD_SCOPE"

// Available reports whether alint is on PATH, so a caller can name the install
// line rather than failing as a mystery spawn error.
func Available() error {
	return shell.Require(Bin)
}

// Check runs every rule over the whole tree, reporting only. It names no
// manifest, which is what keeps a checking run from rewriting anything.
func Check() (Report, error) {
	return run("check", "")
}

// Fix runs the rules the named paths scope, applying what a rule declares a
// fixer for. The manifest is this process's alone and is removed after.
func Fix(paths []string) (Report, error) {
	manifest, err := writeScope(paths)
	if err != nil {
		return Report{}, err
	}

	defer func() { _ = disk.Remove(manifest) }()

	return run("fix", manifest)
}

// scopeEntries is the environment a run adds. A checking run adds nothing:
// ScopeVar is what turns the scoped rules on, and those rules fix.
func scopeEntries(manifest string) []string {
	if manifest == "" {
		return nil
	}

	return []string{ScopeVar + "=" + manifest}
}

// run spawns one subcommand and decodes its verdict. A non-zero exit is a
// violation count, which the report already carries, so it is not an error.
func run(verb, manifest string) (Report, error) {
	result, err := shell.Run(context.Background(), []string{Bin, verb, "--format", "json"},
		shell.Options{
			Env:    shell.Inherit().Set(scopeEntries(manifest)...),
			Output: shell.Capture(),
		})
	if err != nil {
		return Report{}, fmt.Errorf("running %s %s: %w", Bin, verb, err)
	}

	return decode(result.Stdout)
}
