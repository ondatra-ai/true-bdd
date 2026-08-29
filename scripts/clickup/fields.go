package clickup

import (
	"path"
	"strings"
)

// The custom fields a turn in this package can write. `Scope` is a ClickUp
// `labels` field the MCP layer stringifies, refused 400 FIELD_144 (86cba13av);
// `Good For Agent` is a person's, per ticket-schema.yaml.
const (
	triageScoreField     = "1d43d9f5-99b1-41f9-8250-cfdde01b76e0"
	expectedChangesField = "f0c13f08-1762-4c45-8893-6ac4413d3385"
	triageDateField      = "c5e6a521-c13f-4501-846c-6d46991e66a8"
	triageCommitField    = "21913797-c413-4a92-bfef-dbae6e556e54"
)

// The band the Triage Score dropdown offers, which is also the rubric's.
const (
	scoreFloor   = 1
	scoreCeiling = 10
)

// repoWide is the blast radius of a finding that names no file. Legitimate
// per ticket-schema.yaml, which allows it "when the change really is repo-wide".
const repoWide = "./*"

// fieldPlan is one ticket's custom fields, keyed to its `## ` heading number
// so the filing turn can match a row to the task it just created.
type fieldPlan struct {
	Ticket int `json:"ticket"`
	// The dropdown's option INDEX, not the score: ClickUp addresses a
	// drop_down option by position, so score 7 is index 6. Absent when the
	// finding carries no score, which leaves the field alone.
	TriageScoreOrderindex *int   `json:"triage_score_orderindex,omitempty"`
	ExpectedChanges       string `json:"expected_changes,omitempty"`
	TriageDateMillis      int64  `json:"triage_date_millis"`
	TriageCommit          string `json:"triage_commit,omitempty"`
}

// planFields derives what each ticket's custom fields should hold. Derived
// rather than asked for: every value already follows from the finding or the
// run, and a prompt that asks for them is a prompt that can answer wrong.
func planFields(queue []Finding, taken stamp) []fieldPlan {
	plans := make([]fieldPlan, 0, len(queue))

	for index, finding := range queue {
		row := stampedRow(index, finding, taken)
		row.ExpectedChanges = expectedChanges(finding.File)

		plans = append(plans, row)
	}

	return plans
}

// planStamps is the same for a hand-written deferral, which names no file to
// derive a blast radius from. Expected Changes stays a person's: a `./*` here
// would pass task-handle's scope check without ever having bounded anything.
func planStamps(queue []Finding, taken stamp) []fieldPlan {
	plans := make([]fieldPlan, 0, len(queue))

	for index, finding := range queue {
		plans = append(plans, stampedRow(index, finding, taken))
	}

	return plans
}

// stampedRow is what both plans share: the verdict, and when and against what
// it was reached.
func stampedRow(index int, finding Finding, taken stamp) fieldPlan {
	return fieldPlan{
		Ticket:                index + 1,
		TriageScoreOrderindex: orderindexOf(finding.Score),
		TriageDateMillis:      taken.Millis,
		TriageCommit:          taken.Commit,
	}
}

// orderindexOf maps a score onto its dropdown position, or nil when there is
// no score to map — an unscored finding leaves the field empty rather than
// claiming the bottom of the band.
func orderindexOf(score int) *int {
	if score < scoreFloor || score > scoreCeiling {
		return nil
	}

	index := score - 1

	return &index
}

// expectedChanges is the directory a finding's file sits in, as a glob. Never
// the file itself: an exact path forbids adding the test beside it, which is
// not what the scope check is for (ticket-schema.yaml).
func expectedChanges(file string) string {
	trimmed := strings.TrimSpace(file)
	if trimmed == "" {
		return repoWide
	}

	dir := path.Dir(trimmed)
	if dir == "." || dir == "/" {
		return repoWide
	}

	return dir + "/**"
}
