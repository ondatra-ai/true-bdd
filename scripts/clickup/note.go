package clickup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// shortSHA is how much of the commit the note shows. Deliberate: the durable
// full sha is in the Triage Commit field, and this line is for reading.
const shortSHA = 7

// The two body wordings, each naming what the apply turn was actually told to
// do with the description.
const (
	bodyRewritten = "rewritten against HEAD"
	bodyKept      = "left as it is"
)

const unknownHead = "unknown"

// noteOf is the record one triage leaves: what the sweep wrote, and why.
// Composed here, not asked for — an audit line must not be the model's own
// account of itself. Bold, italic and code are all addTaskComment renders.
func noteOf(was prior, verdict triage.Verdict, taken stamp, from, to string) string {
	var built strings.Builder

	fmt.Fprintf(&built, "**Triage** — HEAD %s\n\n", headOf(taken))
	fmt.Fprintf(&built, "Score: %s\n", moved(priorScore(was.Score), strconv.Itoa(verdict.Score)))
	fmt.Fprintf(&built, "Status: %s\n", moved(from, to))
	fmt.Fprintf(&built, "Body: %s\n\n", bodyState(verdict))

	built.WriteString(textutil.Truncate(strings.TrimSpace(verdict.Reason), diagnosticLimit))

	return built.String()
}

// moved renders a transition, or the one value when nothing moved and when
// there was nothing to move from. An arrow means this sweep changed something.
func moved(from, to string) string {
	if from == "" || from == to {
		return to
	}

	return from + " → " + to
}

// priorScore is the score the ticket carried, or "" when the field was unset
// or came back unusable. No Go check can catch a position reported as a label
// — the two bands overlap at 1-9 — so bodiesPromptTemplate carries that.
func priorScore(was string) string {
	score, err := strconv.Atoi(strings.TrimSpace(was))
	if err != nil || score < scoreFloor || score > scoreCeiling {
		return ""
	}

	return strconv.Itoa(score)
}

// headOf is the commit, shortened for reading. now() leaves it unset when git
// rev-parse fails, and a note naming no tree beats one naming a wrong tree.
func headOf(taken stamp) string {
	if taken.Commit == "" {
		return unknownHead
	}

	if len(taken.Commit) > shortSHA {
		return "`" + taken.Commit[:shortSHA] + "`"
	}

	return "`" + taken.Commit + "`"
}

func bodyState(verdict triage.Verdict) string {
	if refreshed(verdict) {
		return bodyRewritten
	}

	return bodyKept
}
