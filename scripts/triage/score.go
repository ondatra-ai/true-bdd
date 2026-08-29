package triage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// The turn. Plan mode refuses writes at the permission layer rather than by
// asking, so a scorer handed Read cannot become a scorer that edits.
const (
	tools      = "Read,Glob,Grep"
	planMode   = "plan"
	role       = "triage"
	diagnostic = 600
)

const defaultTimeout = 900 * time.Second

// attempts is the one retry a rejected ANSWER gets — never a failed turn,
// which would spend a second timeout to learn what the first one said.
const attempts = 2

// errRejected marks an answer that arrived and was not usable, which is the
// only failure worth asking again about.
var errRejected = errors.New("the triage answer was not usable")

// Score judges one subject against the tree as it stands.
func Score(subject Subject) (Verdict, error) {
	prompt := subject.prompt()

	var last error

	for attempt := range attempts {
		verdict, err := ask(prompt, subject.Refresh)

		switch {
		case err == nil:
			slog.Info("Triaged", "subject", subject.ID,
				"score", verdict.Score, "reason", verdict.Reason)

			return verdict, nil
		case !errors.Is(err, errRejected):
			return Verdict{}, fmt.Errorf("scoring %s: %w", subject.ID, err)
		}

		last = err

		slog.Warn("The triage answer was rejected", "subject", subject.ID,
			"attempt", attempt+1, "error", err)
	}

	return Verdict{}, fmt.Errorf("scoring %s: %w", subject.ID, last)
}

// ask runs one turn and reads its answer back.
func ask(prompt string, refresh bool) (Verdict, error) {
	raw, err := claudecli.RunJSON(prompt, claudecli.Options{
		AllowedTools:   tools,
		PermissionMode: planMode,
		Schema:         verdictSchema,
		Role:           role,
		Timeout:        timeout(),
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("the triage turn failed: %w", err)
	}

	var verdict Verdict

	err = json.Unmarshal(raw, &verdict)
	if err != nil {
		return Verdict{}, fmt.Errorf("%w: invalid JSON (%w):\n%s",
			errRejected, err, textutil.Truncate(string(raw), diagnostic))
	}

	err = verdict.validate(refresh)
	if err != nil {
		return Verdict{}, fmt.Errorf("%w: %w", errRejected, err)
	}

	return verdict, nil
}

// timeout bounds one turn. It reads the repository, so it is a slower turn
// than the blind scorer it replaces.
func timeout() time.Duration {
	seconds, err := strconv.Atoi(os.Getenv("TRIAGE_TIMEOUT"))
	if err != nil || seconds <= 0 {
		return defaultTimeout
	}

	return time.Duration(seconds) * time.Second
}
