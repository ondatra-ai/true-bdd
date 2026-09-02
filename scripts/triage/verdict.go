package triage

import (
	"errors"
	"fmt"
	"strings"
)

// Floor is the line the ticket-shaped callers dispose on: at or above it a
// ticket is kept and refreshed, below it retired or never filed. Not merge's,
// which keeps its own Floors table — under a mandate its ticket floor is 9.
const Floor = 6

// The band the Triage Score dropdown offers, checked rather than trusted.
const (
	minScore = 1
	maxScore = 10
)

// Verdict is one triage turn's answer.
type Verdict struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
	// Description is the refreshed body, and is only ever asked for — or
	// returned — under Subject.Filed.
	Description string `json:"description"`
	// Story is how the code behaves today, what goes wrong and where it is
	// fixed. Asked of every path that CREATES a ticket; a filed one carries
	// its story under `### Why`, inside Description.
	Story string `json:"story"`
}

// verdictSchema is the shape the turn is held to. Spelled out rather than
// built from a map: it is a constant, and a constant that cannot fail to
// encode needs no error path.
const verdictSchema = `{"type":"object",` +
	`"required":["score","reason","description","story"],` +
	`"properties":{` +
	`"score":{"type":"integer","minimum":1,"maximum":10},` +
	`"reason":{"type":"string"},` +
	`"description":{"type":"string"},` +
	`"story":{"type":"string"}}}`

var (
	errScoreOutOfBand = errors.New("score is outside the band")
	errNoReason       = errors.New("no reason given")
	errNoRefresh      = errors.New("no refreshed description given")
	errNoStory        = errors.New("no story given")
)

// validate is what the schema cannot say. The band is restated here because
// --json-schema is an instruction to a model and this is a check on its answer.
func (v Verdict) validate(filed bool) error {
	if v.Score < minScore || v.Score > maxScore {
		return fmt.Errorf("%w: %d is not %d-%d", errScoreOutOfBand, v.Score, minScore, maxScore)
	}

	if strings.TrimSpace(v.Reason) == "" {
		return fmt.Errorf("%w for the score %d", errNoReason, v.Score)
	}

	if filed && v.Score >= Floor && strings.TrimSpace(v.Description) == "" {
		return fmt.Errorf("%w: scored %d, which is at or above the floor of %d",
			errNoRefresh, v.Score, Floor)
	}

	// The same rule for the other branch. A ticket filed without the story is
	// the ticket this field exists to stop being filed, and the turn that read
	// the code is the only one that could have written it.
	if !filed && v.Score >= Floor && strings.TrimSpace(v.Story) == "" {
		return fmt.Errorf("%w: scored %d, which is at or above the floor of %d",
			errNoStory, v.Score, Floor)
	}

	return nil
}
