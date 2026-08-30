// Package alint is the child half of alint's `kind: command` rule: a gate that
// alint runs, once per file it matched.
//
// It is an INBOUND protocol, which is what puts it here rather than beside the
// outbound half in pkg/cli/alint (docs/adr/0006). Everything in pkg/cli/<tool>
// is this repository's argv for a binary it starts; this is a foreign runner
// starting us, and its whole contract —
// which environment carries the matched file, that --fix in the rule's argv is
// how a run is told it may rewrite, that a non-zero exit is the verdict and
// the child's own output is the message — is somebody else's to change.
//
// A gate is a closure over one file. It reads what it needs, narrates through
// its logger, and returns a finding or nil; this package owns the descriptor
// that finding reaches, because alint captures the child's output and there is
// exactly one right way to write into that.
package alint

import (
	"log/slog"
	"os"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

// The environment alint gives a `kind: command` child. There is no fourth
// telling it which subcommand ran: dumping the child's environment under
// `check` and under `fix` produces byte-identical output.
const (
	// PathVar holds the file the rule matched, relative to the repository root.
	PathVar = "ALINT_PATH"
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
	// Path is the matched file, repository-relative. The working directory is
	// the repository root, so it is usable as given.
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

// AlintLint runs gate as the child of a `kind: command` rule, built from the
// environment alint set and the argv the rule declared. A finding goes to the
// Console, which alint captures as the message, and is returned for the exit.
func AlintLint(args []string, gate Gate) error {
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

	found := gate(req, slog.Default().With("rule", req.RuleID, "path", req.Path))
	if found == nil {
		return nil
	}

	console.Println(found.Error())

	return found
}
