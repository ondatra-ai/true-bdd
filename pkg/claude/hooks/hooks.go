// Package hooks is Claude Code's hook protocol: the JSON event a hook is
// handed on stdin, and the JSON verdict it answers with.
//
// It is the mirror of pkg/cli/claude. That package spawns the binary; this is
// what the binary spawns, so the two together are the whole of this
// repository's dealings with Claude Code as a process. A hook command's
// main() supplies a closure — the protocol, including which fields exist,
// when a verdict is even read, and which descriptors carry it, lives here.
package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

// blockDecision is the only decision that carries a payload: `reason` is
// DISCARDED unless `decision` is "block", so a finding sent without one is a
// finding nobody reads.
const blockDecision = "block"

// verdict is what a hook writes on stdout for Claude Code to act on.
type verdict struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// block writes the one verdict that stops a tool call.
func block(reason string) error {
	encoded, err := json.Marshal(verdict{Decision: blockDecision, Reason: reason})
	if err != nil {
		return fmt.Errorf("encoding the verdict: %w", err)
	}

	console.Println(string(encoded))

	return nil
}
