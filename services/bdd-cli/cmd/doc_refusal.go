package cmd

import (
	"fmt"
	"log/slog"
)

// refuseUnresolvedDoc reports like runner.refuseStartup — cobra's own error
// would surface as an unattributed stderr usage dump nothing reads.
func refuseUnresolvedDoc(command, label string, err error) error {
	wrapped := fmt.Errorf("resolve %s: %w", label, err)

	slog.Error("Refusing to start: document unresolvable",
		"command", command,
		"document", label,
		"error", wrapped,
	)

	return wrapped
}
