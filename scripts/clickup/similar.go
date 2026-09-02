package clickup

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// similarPrompt asks which existing tickets a candidate already is.
//
//go:embed similar.txt
var similarPrompt string

// fileableCeiling is the highest identity score a candidate may carry and
// still be filed. The rule is the user's: file only on 1-3, where 10 is
// almost identical.
const fileableCeiling = 3

// The band the turn scores in, and how many rows it may return.
const (
	minIdentity = 1
	maxIdentity = 10
	matchLimit  = 3
)

// bodyCap matches the cap a ticket body is rendered at, so a candidate cannot
// reach the turn longer than the corpus entries it is compared against.
const bodyCap = 4000

// matchesSchema is the shape the turn is held to. An object rather than a
// bare array, following scripts/triage/verdict.go:32.
const matchesSchema = `{"type":"object",` +
	`"required":["matches"],` +
	`"properties":{` +
	`"matches":{"type":"array","maxItems":3,"items":{` +
	`"type":"object","required":["id","score","reason"],` +
	`"properties":{` +
	`"id":{"type":"string"},` +
	`"score":{"type":"integer","minimum":1,"maximum":10},` +
	`"reason":{"type":"string"}}}}}}`

var (
	errTooManyMatches    = errors.New("more matches than were asked for")
	errIdentityOutOfBand = errors.New("identity score is outside the band")
	errNoMatchReason     = errors.New("no reason given for a match")
	errUnknownTicket     = errors.New("a match names a ticket the corpus does not hold")
	errSelfMatch         = errors.New("a match names the candidate itself")
	errRankRejected      = errors.New("the similarity answer was not usable")
)

// Match is one existing ticket the turn judged the candidate against.
type Match struct {
	ID     string `json:"id"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`

	// Read off the corpus after validation, never from the answer: these are
	// facts the dump already holds, and asking for them invites a wrong one.
	Name   string `json:"-"`
	URL    string `json:"-"`
	Status string `json:"-"`
}

// answer is the object the turn returns.
type answer struct {
	Matches []Match `json:"matches"`
}

// ranker judges one candidate against the corpus, taken as a parameter so the
// gate below is reachable from a test — the turn itself is not.
type ranker func(candidate Finding, exclude string) ([]Match, error)

// judge ranks candidates against a corpus that is already on disk.
type judge struct {
	byID map[string]CorpusRow
}

// rank runs one turn per candidate and reads its matches back. exclude is the
// candidate's own id when it is already in the corpus — a `dupes` re-audit
// scores a ticket against itself at 10 otherwise — and empty when filing.
func (j judge) rank(candidate Finding, exclude string) ([]Match, error) {
	prompt := rankPrompt(candidate, exclude)

	var last error

	for attempt := range attempts {
		matches, err := j.ask(prompt, exclude)

		switch {
		case err == nil:
			return matches, nil
		case !errors.Is(err, errRankRejected):
			return nil, fmt.Errorf("ranking %s: %w", candidate.Title, err)
		}

		last = err

		slog.Warn("The similarity answer was rejected", "title", candidate.Title,
			"attempt", attempt+1, "error", err)
	}

	return nil, fmt.Errorf("ranking %s: %w", candidate.Title, last)
}

// ask runs one turn and validates what came back.
func (j judge) ask(prompt, exclude string) ([]Match, error) {
	raw, err := claudecli.RunJSON(prompt, claudecli.Options{
		AllowedTools:   rankTools,
		PermissionMode: rankMode,
		Schema:         matchesSchema,
		Role:           roleClickUp,
		Timeout:        claudeTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("the similarity turn failed: %w", err)
	}

	var got answer

	err = json.Unmarshal(raw, &got)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JSON (%w):\n%s",
			errRankRejected, err, textutil.Truncate(string(raw), diagnosticLimit))
	}

	matches, err := j.resolve(got.Matches, exclude)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRankRejected, err)
	}

	return matches, nil
}

// resolve is what the schema cannot say: the band restated, and every id held
// against the corpus that was actually dumped.
func (j judge) resolve(matches []Match, exclude string) ([]Match, error) {
	if len(matches) > matchLimit {
		return nil, fmt.Errorf("%w: %d, want at most %d",
			errTooManyMatches, len(matches), matchLimit)
	}

	resolved := make([]Match, 0, len(matches))

	for _, match := range matches {
		err := validIdentity(match, exclude)
		if err != nil {
			return nil, err
		}

		row, held := j.byID[match.ID]
		if !held {
			return nil, fmt.Errorf("%w: %s", errUnknownTicket, match.ID)
		}

		match.Name, match.URL, match.Status = row.Name, row.URL, row.Status
		resolved = append(resolved, match)
	}

	return resolved, nil
}

// validIdentity checks one row against the band and the exclusion.
func validIdentity(match Match, exclude string) error {
	if match.Score < minIdentity || match.Score > maxIdentity {
		return fmt.Errorf("%w: %d is not %d-%d",
			errIdentityOutOfBand, match.Score, minIdentity, maxIdentity)
	}

	if strings.TrimSpace(match.Reason) == "" {
		return fmt.Errorf("%w: %s scored %d", errNoMatchReason, match.ID, match.Score)
	}

	if exclude != "" && match.ID == exclude {
		return fmt.Errorf("%w: %s", errSelfMatch, exclude)
	}

	return nil
}

// rankPrompt is the whole turn: the rubric, the corpus folder, the exclusion
// when there is one, and the candidate.
func rankPrompt(candidate Finding, exclude string) string {
	var built strings.Builder

	built.WriteString(similarPrompt)

	fmt.Fprintf(&built, "\nThe folder is %s/ — read it with Glob, Grep and Read.\n", CorpusDir)

	if exclude != "" {
		fmt.Fprintf(&built, `
The candidate is itself already filed, as %s. That file is in the folder.
Never return it: it is the ticket being audited, not a match for it.
`, exclude)
	}

	fmt.Fprintf(&built, `
--- BEGIN CANDIDATE ---
title: %s
file : %s

%s
--- END CANDIDATE ---
`, orUnknown(candidate.Title), orUnknown(candidate.File),
		textutil.Truncate(strings.TrimSpace(candidate.Body), bodyCap))

	return built.String()
}

// top is the highest identity score among the matches, or 0 when there are
// none — which is a candidate nothing resembles, and so a fileable one.
func top(matches []Match) Match {
	var best Match

	for _, match := range matches {
		if match.Score > best.Score {
			best = match
		}
	}

	return best
}
