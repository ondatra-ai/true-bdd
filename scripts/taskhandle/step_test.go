package taskhandle_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

// The report tree and the checklist are two artifacts of one step table. This
// is what stops them drifting apart.
func TestEveryStepHasALabelAndAChecklistRow(t *testing.T) {
	t.Parallel()

	list := taskhandle.NewChecklist()

	for index, step := range taskhandle.Steps() {
		want := strconv.Itoa(index + 1)
		if !strings.HasPrefix(step.Label(), want+" ") {
			t.Errorf("%v labelled %q, want it to open with %q", step, step.Label(), want)
		}

		if step.Name() == "unknown" {
			t.Errorf("step %d has no name", index+1)
		}

		list.Done(step, "")
	}

	rendered := list.Render()
	for _, step := range taskhandle.Steps() {
		if !strings.Contains(rendered, "- "+step.Label()) {
			t.Errorf("the checklist has no row for %q", step.Label())
		}
	}
}

func TestStepsAreEightAndOrdered(t *testing.T) {
	t.Parallel()

	steps := taskhandle.Steps()
	if len(steps) != 8 {
		t.Fatalf("got %d steps, want 8", len(steps))
	}

	if steps[0] != taskhandle.StepCheck || steps[7] != taskhandle.StepClose {
		t.Errorf("steps run %v..%v, want Check..Close", steps[0], steps[7])
	}
}
