// Package alint is the child half of alint's `kind: command` rule: a gate that
// alint runs, once per file it matched.
//
// It is an INBOUND protocol, which is what puts it here rather than beside the
// outbound half in pkg/cli/alint (docs/adr/0006). Everything in pkg/cli/<tool>
// is this repository's argv for a binary it starts; this is a foreign runner
// starting us, and its whole contract — which environment carries the matched
// file, that --fix in the rule's argv is how a run is told it may rewrite,
// that a non-zero exit is the verdict and the child's own output is the
// message — is somebody else's to change.
//
// A gate is a closure over one file. It reads what it needs, narrates through
// its logger, and returns a finding or nil; everything around that — the log,
// the working directory, the descriptor the finding reaches, the exit code —
// is this package's, so a command supplies the closure and nothing else.
package alint

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// The environment alint gives a `kind: command` child. There is no fifth
// telling it which subcommand ran: dumping the child's environment under
// `check` and under `fix` produces byte-identical output.
const (
	// PathVar holds the file the rule matched, relative to the repository root.
	PathVar = "ALINT_PATH"
	// RootVar holds the absolute root alint walked, which is the working
	// directory it promises a child — and the one this package restores.
	RootVar = "ALINT_ROOT"
	// RuleVar holds the id of the rule that spawned this child.
	RuleVar = "ALINT_RULE_ID"
	// LevelVar holds that rule's severity.
	LevelVar = "ALINT_LEVEL"
)

// FixFlag is how a rule tells its gate that this run may rewrite. It is in the
// rule's argv because alint has no command-shaped fix operation and exports no
// mode variable, so the config is the only place left to say it.
const FixFlag = "--fix"

// AlintLintParams is one invocation: the file alint matched, what the rule
// asked for, and whether this run may rewrite it.
type AlintLintParams struct {
	// Path is the matched file, repository-relative. Empty when nothing
	// spawned this from a rule, which every gate reads as "the whole tree".
	Path string
	// Args is the rule's argv with FixFlag removed — a gate name, {dir}, or
	// whatever else the rule chose to pass.
	Args []string
	// Fix reports whether FixFlag was present.
	Fix bool
	// RuleID and Level are the rule that spawned this child.
	RuleID string
	Level  string
}

// Gate answers for one request. A nil error is a pass and says nothing; a
// non-nil one is the finding, and its message is what alint shows.
type Gate func(req AlintLintParams, log *slog.Logger) error

// AlintLint is the whole of being a `kind: command` child: bind the log, enter
// the repository, build the request, ask gate, write what it found.
//
//	tool     names the writer in the Task's shared log
//	taskLog  where that log lives — pkg/ may not import the scripts/ package
//	         that names it, so only the caller can say
func AlintLint(tool, taskLog string, args []string, gate Gate) {
	// Stderr always: alint captures this child's output as the violation's
	// message, so narration on stdout would arrive as part of the finding.
	logging.Install(logging.Stderr, taskLog, tool)

	err := answer(args, gate)
	if err == nil {
		return
	}

	// A non-zero exit is the verdict alint reads, so it is owned here.
	os.Exit(1)
}

// answer runs one gate and writes its finding where alint will capture it.
func answer(args []string, gate Gate) error {
	err := enter()
	if err != nil {
		slog.Error("entering the repository", "error", err)

		return err
	}

	req := request(args)

	found := gate(req, slog.Default().With("rule", req.RuleID, "path", req.Path))
	if found == nil {
		return nil
	}

	console.Println(found.Error())

	return found
}

// request reads one invocation out of the environment alint set and the argv
// the rule declared.
func request(args []string) AlintLintParams {
	req := AlintLintParams{
		Path:   os.Getenv(PathVar),
		Fix:    slices.Contains(args, FixFlag),
		RuleID: os.Getenv(RuleVar),
		Level:  os.Getenv(LevelVar),
	}

	for _, arg := range args {
		if arg != FixFlag {
			req.Args = append(req.Args, arg)
		}
	}

	return req
}

// enter restores the working directory alint promises a child. git answers for
// every other caller — the gate table, and a developer at a prompt.
func enter() error {
	root := os.Getenv(RootVar)

	if root == "" {
		top, err := git.TopLevel()
		if err != nil {
			return fmt.Errorf("finding the repository root: %w", err)
		}

		root = top
	}

	resolved, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", root, err)
	}

	err = os.Chdir(resolved)
	if err != nil {
		return fmt.Errorf("moving to %s: %w", resolved, err)
	}

	return nil
}
