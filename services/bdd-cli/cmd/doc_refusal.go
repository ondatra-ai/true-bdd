package cmd

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/console"
)

// refuseUnresolvedDoc reports like runner.refuseStartup: console for the
// operator, slog for the BDD judge/harness — cobra's own error would
// surface as an unattributed stderr usage dump that neither channel reads.
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
