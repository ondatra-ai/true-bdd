package triage

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// bodyLimit caps how much of a subject reaches the turn. It holds a WHOLE
// rendered ticket — two 4,000-rune fields plus its headings — because a
// refresh returns what it was shown, so a tail cut here is a tail deleted.
const bodyLimit = 10000

// Subject is one claim about this repository, awaiting judgement.
type Subject struct {
	// ID is the caller's handle — a ticket id, a finding id. Echoed in
	// diagnostics; the turn never sees it as anything but a label.
	ID       string
	Title    string
	Body     string
	File     string
	Line     string
	Origin   string
	Severity string

	// Filed marks a subject that IS a ticket the tracker already holds: rewrite
	// it against HEAD, and never grow it into a newer shape. Set by the sweep
	// alone. Every creating path leaves it unset and is asked for a story.
	Filed bool
}

// prompt is the whole turn: the rubric, the clause that says where the
// narrative goes, and the subject. Filed is the one branch — the two clauses
// are the two homes a story can have, not a second axis.
func (s Subject) prompt() string {
	var built strings.Builder

	built.WriteString(rubricPrompt)

	if s.Filed {
		built.WriteString(refreshPrompt)
	} else {
		built.WriteString(storyPrompt)
	}

	fmt.Fprintf(&built, `
--- BEGIN SUBJECT ---
origin   : %s
file     : %s:%s
severity : %s — whoever raised it said so; score by what you read, not by this
title    : %s

%s
--- END SUBJECT ---
`, orUnknown(s.Origin), orUnknown(s.File), orUnknown(s.Line), orUnknown(s.Severity),
		orUnknown(s.Title), textutil.Truncate(strings.TrimSpace(s.Body), bodyLimit))

	return built.String()
}

// orUnknown keeps an absent field readable: `?` in a labelled block is legible
// where a trailing colon is not.
func orUnknown(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "?"
	}

	return trimmed
}
