package reporter

import (
	"fmt"
	"strconv"
	"strings"
)

// TurnCause is why a turn happened, in structured form. The wording
// lives in Phrase so the classifier can stay about evidence and the
// label about English.
type TurnCause struct {
	Kind string
	// WalkAttempt and MaxAttempts locate a re-walk: "re-walk 2/5".
	WalkAttempt int
	MaxAttempts int
	// FixNumber is the run-global apply counter (runner.go fixCount) —
	// the same number the `apply-<subject>-iterN` artifacts carry.
	FixNumber int
	// Round is the fix generator's per-cell iteration, already folded
	// back to a human count: clarification round N, or refinement N.
	Round int
}

// Operation is what a turn did, named. Every field is derived from the run's
// own log and its own copy of the checklist, so `build code` needs no
// special case.
type Operation struct {
	Verb    string
	Section string
	Subject string
	// Ref names what the turn worked from: `Q[1]` for a validation,
	// `F[1]` for a fix generation, the generated fix-prompt artifact for
	// an apply. See turnRef for why a validation never names F.
	Ref string
	// Why is the cause clause: "walk 1", "after fix #1 was applied".
	Why string
	// Label is the full attempt-aware name.
	Label string
	// CellLabel is Label with the first-entry verb, always: cross-run
	// comparison aligns on the cell, not the attempt, so a matched row can
	// pair attempt 1 with attempt 2 without "Re-validate" claiming one side.
	CellLabel string
	// SectionName is the checklist author's human name for the section
	// ("Scenario Merge"). Shown in the turn's fact table, not the row:
	// the row uses the id, which is what the filenames carry.
	SectionName string
}

// firstReWalk is the lowest walk number that can BE a re-walk: walk 1 is
// the original pass. A cause carrying anything less simply never had its
// walk recorded.
const firstReWalk = 2

// Phrase words a cause. An empty string means the cause is unknown — normal
// for a session recorded before the engine logged walk boundaries — so the
// label omits the clause rather than guessing.
func (c TurnCause) Phrase(promptRef string) string {
	switch c.Kind {
	case causeWalk:
		// A first entry into a cell can only happen on the first walk
		// when the walk number is unrecorded: every later walk re-walks
		// every item, so nothing is being seen for the first time there.
		return "walk " + strconv.Itoa(max(c.WalkAttempt, 1))
	case causeReWalk:
		// No count rather than a made-up "2/5" when WalkAttempt wasn't recorded.
		if c.WalkAttempt < firstReWalk {
			return "re-walk — the previous walk applied a fix"
		}

		return fmt.Sprintf("re-walk %d/%d — fixes applied in walk %d",
			c.WalkAttempt, c.MaxAttempts, c.WalkAttempt-1)
	case causeAfterFix:
		if c.FixNumber > 0 {
			return fmt.Sprintf("after fix #%d was applied", c.FixNumber)
		}

		return "after a fix was applied"
	case causeUserApply:
		if c.FixNumber > 0 {
			return fmt.Sprintf("user applied fix #%d", c.FixNumber)
		}

		return "user chose [1] apply"
	case causeJudge:
		return "harness verdict"
	default:
		return c.fixPhrase(promptRef)
	}
}

// fixPhrase words the three reasons a fix turn runs. Split out so the
// two loops the engine runs — clarification and refinement — read as one
// family rather than as more arms of the outer switch.
func (c TurnCause) fixPhrase(promptRef string) string {
	switch c.Kind {
	case causeCheckFailed:
		if promptRef != "" {
			return promptRef + " answered fail"
		}

		return "the check answered fail"
	case causeClarify:
		return fmt.Sprintf("clarification round %d", c.Round)
	case causeRefine:
		return fmt.Sprintf("user refinement %d", c.Round)
	default:
		return ""
	}
}

// describeTurn names one turn. command is the hyphenated checklist command
// the run logged ("us-apply"), used only to strip the prefix the artifact
// filenames glue onto the section id.
func describeTurn(turn *Turn, doc ChecklistDoc, command string) Operation {
	index := turn.PromptIndex()
	entry, known := doc.Prompt(index)

	operation := Operation{
		Subject:     turn.Cell.Subject,
		Section:     sectionID(turn.Cell.Section, command, entry, known),
		SectionName: entry.SectionName,
		Ref:         turnRef(turn, index, known && entry.HasFix),
	}

	operation.Verb = verbFor(turn.Role, turn.Attempt)
	operation.Why = turn.Cause.Phrase(questionRef(index))
	operation.Label = assemble(operation.Verb, operation, turn.Role)
	operation.CellLabel = assemble(verbFor(turn.Role, 1), operation, turn.Role)

	return operation
}

// verbFor picks the verb from the role and whether this is a re-entry.
// attempt is 1-based within (cell, role); anything past the first is a
// repeat, and saying so is the point of the whole exercise.
func verbFor(role string, attempt int) string {
	repeat := attempt > 1

	switch role {
	case rolePrompt:
		if repeat {
			return verbReValidate
		}

		return verbValidate
	case roleFix:
		if repeat {
			return verbRegenFix
		}

		return verbGenFix
	case roleApply:
		if repeat {
			return verbReApplyFix
		}

		return verbApplyFix
	case roleJudge:
		return verbJudge
	default:
		return role
	}
}

// turnRef names what the turn was working from.
//
//	VALIDATE  Q[n] — the F: template is cargo for a possible fail, not what
//	          the check is measured against; naming it here would imply the
//	          validation had something to do with fixing.
//	FIX       F[n] — receives F's rendered output as "Suggested Fix Template".
//	APPLY     the generated fix prompt only; it never sees F, so an F index
//	          here would be false.
func turnRef(turn *Turn, index int, hasFix bool) string {
	switch turn.Role {
	case rolePrompt:
		return questionRef(index)
	case roleFix:
		if index == 0 {
			return ""
		}

		if hasFix {
			return fixRef(index)
		}

		return questionRef(index)
	case roleApply:
		return turn.FixPromptArtifact
	default:
		return ""
	}
}

func questionRef(index int) string {
	if index == 0 {
		return ""
	}

	return "Q[" + strconv.Itoa(index) + "]"
}

func fixRef(index int) string {
	return "F[" + strconv.Itoa(index) + "]"
}

// sectionID prefers the id the checklist itself declares and falls back
// to stripping the command prefix off the flattened section path the
// filename carries ("us-apply-merge" → "merge").
func sectionID(section, command string, entry ChecklistPrompt, known bool) string {
	if known && entry.SectionID != "" {
		return entry.SectionID
	}

	if command != "" {
		return strings.TrimPrefix(section, command+"-")
	}

	return section
}

// assemble builds the row text as an English sentence, one shape per role.
//
//	Validate            99.3-001 against Q[1]
//	Generate fix prompt for 99.3-001 from F[1]
//	Apply fix           to 99.3-001 from 01-99.3-001-fix-prompts.md
func assemble(verb string, operation Operation, role string) string {
	target := operation.Subject

	preposition, source := " against ", operation.Ref

	switch role {
	case roleFix:
		if target != "" {
			target = "for " + target
		}

		preposition = " from "
	case roleApply:
		if target != "" {
			target = "to " + target
		}

		preposition = " from "
	}

	label := strings.TrimSpace(verb + " " + target)
	if source == "" {
		return label
	}

	return label + preposition + source
}
