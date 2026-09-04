package reporter

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CellRoleKey is a turn's identity as "which seat of which cell": the
// checklist cell plus which role this turn occupied. The iteration index is
// deliberately excluded, so retries of the same seat share a key.
func (t *Turn) CellRoleKey() string {
	section := t.Cell.Section
	if section == "" {
		section = "?"
	}

	subject := t.Cell.Subject
	if subject == "" {
		subject = "?"
	}

	return strings.Join([]string{section, subject, t.Role}, "\x1f")
}

// seatKey is CellRoleKey plus the prompt index, and it is what attempts are
// counted on — unlike CellRoleKey, since a section can hold several
// prompts (us-create's `format` holds two) that are different questions, not retries of one.
func (t *Turn) seatKey() string {
	return t.CellRoleKey() + "\x1f" + strconv.Itoa(t.PromptIdx)
}

// turnEvidence is the log state at the moment a turn was dispatched:
// what the run had already done, held so the cause can be read off it
// once attempts are known.
type turnEvidence struct {
	walk      int
	maxWalk   int
	fixNumber int
	// lastFixForSubject is the run-global fix count most recently
	// applied to THIS turn's subject when it was dispatched.
	lastFixForSubject int
	fixIteration      int
}

// subjectUnsafe mirrors the engine's sanitizeID
// (src/internal/app/generators/validate/id_sanitizer.go): every run of
// characters outside the safe set collapses to a single "-".
var subjectUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sanitizeSubject makes a logged subject id comparable with the one parsed
// out of a filename: the log carries the raw id, the filename a flattened
// one with the trailing hyphen trimmed by cellFromArtifact — both must agree.
func sanitizeSubject(id string) string {
	return strings.Trim(subjectUnsafe.ReplaceAllString(id, "-"), "-")
}

// contextWalker replays the record stream, carrying the run state each
// turn needs to explain itself. It claims one turn per dispatch, which
// is how turnFolder built the turn list from this same stream.
type contextWalker struct {
	turns  []*Turn
	cursor int

	walk      int
	maxWalk   int
	docs      []string
	docsIdx   int
	fixIter   int
	applyFix  int
	fixPrompt string
	// lastFix is the run-global fix count most recently applied to a
	// subject; lastGenIdx is the prompt index of its most recent fix
	// generation, which the apply turn's own artifacts never carry.
	lastFix    map[string]int
	lastGenIdx map[string]int
}

// resolveTurnContext gives each turn the prompt index its artifacts may
// not carry, the documents it resolved, and the run state it was
// dispatched under.
func resolveTurnContext(records []LogRecord, turns []*Turn) {
	walker := &contextWalker{
		turns:      turns,
		lastFix:    map[string]int{},
		lastGenIdx: map[string]int{},
	}

	for index := range records {
		walker.consume(&records[index])
	}
}

func (w *contextWalker) consume(record *LogRecord) {
	subject := sanitizeSubject(record.SubjectID)

	switch record.Msg {
	case msgWalkAttempt:
		w.walk = intOr(record.Attempt)
		w.maxWalk = intOr(record.MaxAttempts)
	case msgDocsResolved:
		w.docs = formatDocs(record.Docs)
		w.docsIdx = intOr(record.PromptIndex)
	case msgFixGenerating:
		w.fixIter = intOr(record.Iteration)
		w.lastGenIdx[subject] = intOr(record.PromptIndex)
	case msgFixPromptSaved:
		// The apply turn reads THIS file. Its name comes from the record rather
		// than being rebuilt from the subject, since the engine's own name
		// survives ids that sanitise to a trailing hyphen and a reconstruction wouldn't.
		w.fixPrompt = filepath.Base(record.File)
	case msgFixApplying:
		w.applyFix = intOr(record.Iteration)
	case msgFixApplied:
		w.lastFix[subject] = w.applyFix
	case msgDispatch:
		w.claimTurn()
	}
}

// claimTurn attaches everything gathered since the last dispatch to the
// turn that dispatch started.
func (w *contextWalker) claimTurn() {
	if w.cursor >= len(w.turns) {
		return
	}

	turn := w.turns[w.cursor]
	w.cursor++

	// Both fields come from the same record and are cleared together, so a
	// turn with no documents record of its own reads as "index unknown"
	// rather than inheriting a stale index from an earlier turn.
	turn.Docs = w.docs
	pendingIdx := w.docsIdx
	w.docs, w.docsIdx = nil, 0

	turn.PromptIdx = promptIndexOf(turn, w.lastGenIdx, pendingIdx)
	turn.evidence = turnEvidence{
		walk:              w.walk,
		maxWalk:           w.maxWalk,
		fixNumber:         w.applyFix,
		lastFixForSubject: w.lastFix[turn.Cell.Subject],
		fixIteration:      w.fixIter,
	}

	if turn.Role == roleApply {
		turn.FixPromptArtifact = w.fixPrompt
	}
}

// promptIndexOf resolves a turn's 1-based prompt index. Validation and
// fix artifacts name it; apply artifacts do not, so an apply inherits
// the index of the fix generation it followed.
func promptIndexOf(turn *Turn, lastGenIdx map[string]int, pending int) int {
	index, err := strconv.Atoi(turn.Cell.Index)
	if err == nil && index > 0 {
		return index
	}

	if turn.Role == roleApply {
		return lastGenIdx[turn.Cell.Subject]
	}

	return pending
}

// assignAttempts numbers every turn within its seat: Attempt is 1-based
// and this-one-included, AttemptTotal is how many the run ends up
// making. Attempt > 1 is what the verb reads as a re-entry.
func assignAttempts(turns []*Turn) {
	totals := make(map[string]int, len(turns))
	for _, turn := range turns {
		totals[turn.seatKey()]++
	}

	seen := make(map[string]int, len(turns))

	for _, turn := range turns {
		key := turn.seatKey()
		seen[key]++
		turn.Attempt = seen[key]
		turn.AttemptTotal = totals[key]
	}
}

// classifyCauses explains every turn from the evidence already attached
// to it.
func classifyCauses(turns []*Turn) {
	attributed := map[string]int{}

	for _, turn := range turns {
		switch turn.Role {
		case rolePrompt:
			turn.Cause = promptCause(turn, attributed)
		case roleFix:
			turn.Cause = fixCause(turn.evidence.fixIteration)
		case roleApply:
			turn.Cause = TurnCause{Kind: causeUserApply, FixNumber: turn.evidence.fixNumber}
		}
	}
}

// promptCause separates a first look from a re-entry caused by this item's
// own fix (walker.go) vs. the outer re-walk (engine.go), by arithmetic: every
// fix bumps a run-global counter, so a restart always carries one this cell hasn't seen; no new fix means re-walk.
func promptCause(turn *Turn, attributed map[string]int) TurnCause {
	if turn.Attempt <= 1 {
		return TurnCause{Kind: causeWalk, WalkAttempt: turn.evidence.walk}
	}

	key := turn.seatKey()
	if fix := turn.evidence.lastFixForSubject; fix > attributed[key] {
		attributed[key] = fix

		return TurnCause{Kind: causeAfterFix, FixNumber: fix}
	}

	return TurnCause{
		Kind:        causeReWalk,
		WalkAttempt: turn.evidence.walk,
		MaxAttempts: turn.evidence.maxWalk,
	}
}

// fixCause reads the generator's own iteration counter against
// maxClarificationRounds, which mirrors the engine's own cap (see below).
func fixCause(iteration int) TurnCause {
	switch {
	case iteration <= 1:
		return TurnCause{Kind: causeCheckFailed}
	case iteration <= maxClarificationRounds:
		return TurnCause{Kind: causeClarify, Round: iteration}
	default:
		return TurnCause{Kind: causeRefine, Round: iteration - maxClarificationRounds}
	}
}

// maxClarificationRounds mirrors the engine's own cap
// (src/internal/app/engine/types.go maxClarificationIterations). A fix
// iteration past it is a user refinement, not a clarification.
const maxClarificationRounds = 5

// formatDocs renders the resolved document set as sorted "key → path"
// lines, stable enough to diff between two runs.
func formatDocs(docs map[string]string) []string {
	if len(docs) == 0 {
		return nil
	}

	lines := make([]string, 0, len(docs))
	for key, path := range docs {
		lines = append(lines, key+" → "+path)
	}

	sort.Strings(lines)

	return lines
}
