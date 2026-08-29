package clickup

import (
	"context"
	"log/slog"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
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

	sha, err := git.HeadSHA(context.Background())
	if err != nil {
		slog.Warn("Could not read HEAD; the triage commit will be left unset", "error", err)

		return taken
	}

	taken.Commit = sha

	return taken
}
