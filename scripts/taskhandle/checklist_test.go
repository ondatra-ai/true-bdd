package taskhandle_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

// A halt at step 3 must not print five bare `–` lines; the spec collapses them.
func TestRenderCollapsesTheUnreachedTail(t *testing.T) {
	t.Parallel()

	list := taskhandle.NewChecklist()
	list.Done(taskhandle.StepCheck, "")
	list.Done(taskhandle.StepStart, "bound")
	list.Fail(taskhandle.StepWork, "the plan turn failed")

	got := list.Render()

	if !strings.HasSuffix(got, "- 4–8 – not reached") {
		t.Errorf("render ended %q, want the tail collapsed into one range line", got)
	}

	if strings.Count(got, "\n") != 3 {
		t.Errorf("render has %d lines, want 4:\n%s", strings.Count(got, "\n")+1, got)
	}
}

// A bare ✗ is not a report.
func TestRenderRefusesAMarkerWithNoNote(t *testing.T) {
	t.Parallel()

	for name, mark := range map[string]func(*taskhandle.Checklist, taskhandle.Step, string){
		"failed":  (*taskhandle.Checklist).Fail,
		"warned":  (*taskhandle.Checklist).Warn,
		"skipped": (*taskhandle.Checklist).Skip,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list := taskhandle.NewChecklist()
			mark(list, taskhandle.StepCheck, "")

			if !strings.Contains(list.Render(), "no note") {
				t.Errorf("a bare marker rendered as %q", list.Render())
			}
		})
	}
}

// A ✓ says everything the outcome line already says, so it needs no note.
func TestRenderAllowsABareTick(t *testing.T) {
	t.Parallel()

	list := taskhandle.NewChecklist()
	list.Done(taskhandle.StepCheck, "")

	if got := firstLine(list.Render()); got != "- 1 Check ✓" {
		t.Errorf("render opened %q, want a bare tick", got)
	}
}

func TestRenderWalksEveryStepInOrder(t *testing.T) {
	t.Parallel()

	list := taskhandle.NewChecklist()
	for _, step := range taskhandle.Steps() {
		list.Done(step, "")
	}

	lines := strings.Split(list.Render(), "\n")
	if len(lines) != len(taskhandle.Steps()) {
		t.Fatalf("got %d lines, want %d", len(lines), len(taskhandle.Steps()))
	}

	for index, step := range taskhandle.Steps() {
		if !strings.HasPrefix(lines[index], "- "+step.Label()) {
			t.Errorf("line %d = %q, want it to open with %q", index, lines[index], step.Label())
		}
	}
}

// A run that got nowhere still renders something a reader can act on.
func TestRenderOfAnEmptyChecklist(t *testing.T) {
	t.Parallel()

	if got := taskhandle.NewChecklist().Render(); got != "- 1–8 – not reached" {
		t.Errorf("render = %q", got)
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return line
}
