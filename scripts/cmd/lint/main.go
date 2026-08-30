// Command lint is this repository's source-quality gates, as the child alint
// runs and as the command CI runs directly.
//
// One closure answers both. alint sets ALINT_PATH and puts --fix in the rule's
// argv, so a gate named with neither is the whole-repository check the gate
// table asks for. It never spawns alint — that is ./scripts/cmd/alint_hook,
// and a leaf that ran its own runner would loop.
//
//	lint <gate>                      every tracked file, report only
//	lint <gate> <file>...            those files
//	lint golint --fix                golangci-lint over ALINT_PATH's package
//	lint comments                    the comment budget, whole tree
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
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/lint"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// errUnknownGate names a gate no branch below answers for.
var errUnknownGate = errors.New("unknown gate")

// finding is a gate's report as an error: alint shows the message verbatim,
// so a wrapped static error would prefix the thing the reader must act on.
type finding string

func (f finding) Error() string { return string(f) }

// tool names this program in the Task's shared log.
const tool = "lint"

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
