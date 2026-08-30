// Command linters is this repository's source-quality leaves, and is run by
// alint alone — every rule in .alint.yml that names a linter names this.
//
// Nothing else may call it: `alint` is the one entry point, so which file
// selects which leaf stays a fact of .alint.yml rather than a second table.
// It never spawns alint back — a leaf that ran its own runner would loop.
//
//	linters <gate>                   every tracked file, report only
//	linters <gate> <file>...         those files
//	linters golint --fix             golangci-lint over ALINT_PATH's package
//	linters comments                 the comment budget, whole tree
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/alint"
	clialint "github.com/ondatra-ai/true-bdd/pkg/cli/alint"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/lint"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// allGate is the whole-tree run: every leaf once, which is what a gate wants
// and what a per-file command rule cannot express.
const allGate = "all"

// errUnknownGate names a gate no branch below answers for.
var errUnknownGate = errors.New("unknown gate")

// finding is a gate's report as an error: alint shows the message verbatim,
// so a wrapped static error would prefix the thing the reader must act on.
type finding string

func (f finding) Error() string { return string(f) }

// tool names this program in the Task's shared log.
const tool = "linters"

func main() {
	alint.AlintLint(tool, state.TaskLog(history.RepoRoot()), os.Args[1:], gate)
}

// gate is the one closure: it picks the check the first argument names and
// hands back what that check found, which alint reads as the verdict.
func gate(req alint.AlintLintParams, log *slog.Logger) error {
	if len(req.Args) == 0 {
		return fmt.Errorf("%w: name one", errUnknownGate)
	}

	name, rest := req.Args[0], req.Args[1:]

	if name == allGate {
		return everything(log)
	}

	files := scope(req, rest)

	return capture(log, name, func(out *bytes.Buffer) error {
		switch name {
		case "comments":
			return lint.Comments(out, files)
		case "schemas":
			return lint.Schemas(out, files)
		case "markdown":
			return lint.Markdown(out, files)
		case "claude-md":
			return lint.ClaudeMD(out)
		case "golint":
			return lint.GoPackage(out, packages(files, rest), req.Fix)
		case "eslint":
			return lint.ESLint(out, files, req.Fix)
		default:
			return fmt.Errorf("%w: %s", errUnknownGate, name)
		}
	})
}

// everything runs every leaf ONCE over the whole tree — the gate's shape, as
// against the scoped rules' one-file-per-invocation. Inert under a scope: the
// hook's own rules already cover the edited file, and this would re-walk all.
func everything(log *slog.Logger) error {
	// The name is pkg/cli/alint's, imported for the constant alone: one spelling
	// of the variable that decides scoped-versus-whole-tree.
	if os.Getenv(clialint.ScopeVar) != "" {
		return nil
	}

	var failures []string

	for _, leaf := range []struct {
		name string
		run  func(out *bytes.Buffer) error
	}{
		{"golint", func(out *bytes.Buffer) error { return lint.GoPackage(out, nil, false) }},
		{"comments", func(out *bytes.Buffer) error { return lint.Comments(out, nil) }},
		{"markdown", func(out *bytes.Buffer) error { return lint.Markdown(out, nil) }},
		{"schemas", func(out *bytes.Buffer) error { return lint.Schemas(out, nil) }},
		{"eslint", func(out *bytes.Buffer) error { return lint.ESLint(out, nil, false) }},
	} {
		err := capture(log, leaf.name, leaf.run)
		if err != nil {
			failures = append(failures, leaf.name+":\n"+err.Error())
		}
	}

	if len(failures) > 0 {
		return finding(strings.Join(failures, "\n\n"))
	}

	return nil
}

// scope is the files this run answers for: the one alint matched, else the
// ones the argv named, else none — which every gate reads as "the whole tree".
func scope(req alint.AlintLintParams, named []string) []string {
	if req.Path != "" {
		return []string{req.Path}
	}

	return named
}

// packages names the Go packages a run covers: the directories holding the
// files alint matched, or the ones the argv named outright.
func packages(files, named []string) []string {
	if len(named) > 0 {
		return named
	}

	dirs := make([]string, 0, len(files))
	for _, file := range files {
		dirs = append(dirs, filepath.Dir(file))
	}

	return dirs
}

// capture turns a gate's report into the closure's verdict: a pass narrates,
// a failure hands the findings back for pkg/alint to write.
func capture(log *slog.Logger, name string, run func(*bytes.Buffer) error) error {
	var found bytes.Buffer

	err := run(&found)
	report := strings.TrimSpace(found.String())

	if err == nil {
		log.Info("gate passed", "gate", name, "report", report)

		return nil
	}

	if report == "" {
		return err
	}

	return finding(report)
}
