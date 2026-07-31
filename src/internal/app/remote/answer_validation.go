package remote

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
)

// Remote-side answer hygiene (plan §3.2): the server already validates
// browser answers, but the remote independently guards its child's stdin
// so a malformed answer never corrupts the line-based collector protocol.
const (
	// maxChoiceBytes caps a choice answer (apply|refine|exit or 1|2|3).
	maxChoiceBytes = 16
	// maxClarifyBytes caps a single-line clarify answer.
	maxClarifyBytes = 4096
	// maxFreetextBytes caps a multiline freetext answer (payload cap).
	maxFreetextBytes = 64 * 1024
)

// The apply/refine/exit choice enum (mirrors the engine's UserAction) the
// browser sends for a choice prompt.
const (
	answerApply  = "apply"
	answerRefine = "refine"
	answerExit   = "exit"
)

// errInvalidAnswer is the sentinel wrapped with the concrete violation.
var errInvalidAnswer = errors.New("invalid answer for prompt")

// choiceValues are the answer strings a choice prompt accepts: the
// canonical apply/refine/exit enum plus the 1/2/3 numeric equivalents the
// collector also maps (user_input_collector.go).
//
//nolint:gochecknoglobals // fixed answer enum
var choiceValues = map[string]bool{
	answerApply: true, answerRefine: true, answerExit: true,
	"1": true, "2": true, "3": true,
}

// validateAnswer rejects an answer that would corrupt the child's stdin
// protocol for its prompt kind: a choice outside the enum, a clarify or
// choice value carrying a newline, or an over-cap payload.
func validateAnswer(kind, value string) error {
	switch kind {
	case string(events.KindChoice):
		return validateChoice(value)
	case string(events.KindClarify):
		return validateSingleLine(value, maxClarifyBytes)
	case string(events.KindFreetext):
		return validateFreetext(value)
	default:
		return fmt.Errorf("%w: unknown prompt kind %q", errInvalidAnswer, kind)
	}
}

// validateChoice enforces the apply/refine/exit enum (or its numeric
// equivalent).
func validateChoice(value string) error {
	if len(value) > maxChoiceBytes || !choiceValues[value] {
		return fmt.Errorf("%w: choice %q not in {apply,refine,exit}", errInvalidAnswer, value)
	}

	return nil
}

// validateSingleLine rejects newlines and over-cap single-line answers.
func validateSingleLine(value string, maxBytes int) error {
	if len(value) > maxBytes {
		return fmt.Errorf("%w: answer exceeds %d bytes", errInvalidAnswer, maxBytes)
	}

	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%w: single-line answer contains a newline", errInvalidAnswer)
	}

	return nil
}

// validateFreetext caps the multiline payload; newlines are legal.
func validateFreetext(value string) error {
	if len(value) > maxFreetextBytes {
		return fmt.Errorf("%w: freetext exceeds %d bytes", errInvalidAnswer, maxFreetextBytes)
	}

	return nil
}
