package taskhandle

import "strconv"

// Step is one of the eight a run walks. The number is the report node's label
// AND the checklist row's, from this one table, so the two cannot disagree
// about what step 5 is called.
type Step int

const (
	StepCheck Step = iota + 1
	StepStart
	StepWork
	StepScope
	StepCommit
	StepReview
	StepMerge
	StepClose
)

//nolint:gochecknoglobals // the step table; Go has no const slice.
var stepNames = []string{
	"Check", "Start", "Work", "Scope check",
	"Commit", "Review", "Merge", "Close",
}

// Name is the step's title.
func (s Step) Name() string {
	if s < StepCheck || int(s) > len(stepNames) {
		return "unknown"
	}

	return stepNames[s-1]
}

// Label is what report.Open stamps and the checklist prints: "5 Commit".
func (s Step) Label() string {
	return strconv.Itoa(int(s)) + " " + s.Name()
}

// Steps is every step, in order.
func Steps() []Step {
	all := make([]Step, 0, len(stepNames))
	for index := range stepNames {
		all = append(all, Step(index+1))
	}

	return all
}
