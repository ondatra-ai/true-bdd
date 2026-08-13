package cmd

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/src/internal/pkg/console"
)

// refuseUnresolvedDoc reports a cmd-layer document-resolution failure
// the way runner.Run reports an unsatisfiable checklist document (see
// src/internal/app/runner/runner.go). Both channels, deliberately: the
// console line is for whoever ran the command, the log record is what a
// harness, a CI scrape, or the BDD judge reads afterwards.
//
// The `us` and `build` commands resolve their spec paths before the
// runner is ever entered, so the runner's own refusal never fires for
// them. Left to cobra, the error surfaces as a stderr line under a
// usage dump — the output shape for "you typed the flags wrong", not
// for "your host config is incomplete" — and nothing at all reaches
// stdout or the log. A refusal nobody can attribute is barely better
// than the silent fallback the configured path replaced.
//
// The returned error is exactly what the call sites built before, so
// wrapping and exit code are unchanged; only the reporting is added.
func refuseUnresolvedDoc(command, label string, err error) error {
	wrapped := fmt.Errorf("resolve %s: %w", label, err)

	slog.Error("Refusing to start: document unresolvable",
		"command", command,
		"document", label,
		"error", wrapped,
	)
	console.Println("Cannot start: " + wrapped.Error())

	return wrapped
}
