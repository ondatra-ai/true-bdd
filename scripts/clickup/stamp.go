package clickup

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// stamp is when a triage decision was taken and the tree it was taken
// against. Both, or the record cannot answer "is this score still true?" —
// a date alone does not name what the ticket was read against.
type stamp struct {
	Millis int64
	Commit string
}

// now reads the stamp for this run. A commit that cannot be read is a warning,
// not a stop: the tickets are still worth filing, and the field is left alone
// rather than filled with a guess.
func now() stamp {
	taken := stamp{Millis: time.Now().UnixMilli()}

	head := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")

	sha, err := head.Output()
	if err != nil {
		slog.Warn("Could not read HEAD; the triage commit will be left unset", "error", err)

		return taken
	}

	taken.Commit = strings.TrimSpace(string(sha))

	return taken
}
