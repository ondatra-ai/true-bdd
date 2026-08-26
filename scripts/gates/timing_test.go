package gates_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

// first is named three times, and succeed is every cheap gate's command.
const (
	first   = "first"
	succeed = "true"
)

// table is three cheap gates: the timing behaviour without the real pipeline.
//
//nolint:gochecknoglobals // the fixture this file is about.
var table = []gates.Gate{
	{Name: first, Command: []string{succeed}},
	{Name: "second", Command: []string{succeed}},
	{Name: "third", Command: []string{succeed}},
}

func TestRunTimesEveryGateItRan(t *testing.T) {
	t.Parallel()

	timings, err := gates.Run(io.Discard, table)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(timings) != len(table) {
		t.Fatalf("timed %d gates, ran %d", len(timings), len(table))
	}

	for index, timing := range timings {
		if timing.Name != table[index].Name {
			t.Errorf("timing %d is %q, want %q", index, timing.Name, table[index].Name)
		}
	}
}

// A red gate's duration is the one the reader wants most, and a run that
// returned nothing would report it as never reached.
func TestRunTimesTheGateThatFailed(t *testing.T) {
	t.Parallel()

	timings, err := gates.Run(io.Discard, []gates.Gate{
		{Name: "green", Command: []string{succeed}},
		{Name: "red", Command: []string{"false"}},
		{Name: "never", Command: []string{succeed}},
	})
	if err == nil {
		t.Fatal("a failing gate returned no error")
	}

	if len(timings) != 2 {
		t.Fatalf("timed %d gates, want the green one and the red one", len(timings))
	}

	if timings[1].Name != "red" {
		t.Errorf("last timing is %q, want the failed gate", timings[1].Name)
	}
}

func TestRenderSummaryNamesEveryGateSkippedOrTimed(t *testing.T) {
	t.Parallel()

	summary := render(table, table[:1], []gates.Timing{{Name: first, Elapsed: 2 * time.Second}})

	for _, gate := range table {
		if !strings.Contains(summary, gate.Name) {
			t.Errorf("the summary does not name gate %q:\n%s", gate.Name, summary)
		}
	}

	if strings.Count(summary, "skipped") != 2 {
		t.Errorf("want the two unselected gates marked skipped:\n%s", summary)
	}

	if !strings.Contains(summary, "2.0") {
		t.Errorf("the summary does not carry the gate's duration:\n%s", summary)
	}
}

// A selected gate with no timing is one the first failure stopped the pipeline
// before — different from one the diff did not need.
func TestRenderSummarySeparatesNotReachedFromSkipped(t *testing.T) {
	t.Parallel()

	summary := render(table, table, []gates.Timing{{Name: first, Elapsed: time.Second}})

	if strings.Contains(summary, "skipped") {
		t.Errorf("nothing was skipped, yet the summary says so:\n%s", summary)
	}

	if strings.Count(summary, "not reached") != 2 {
		t.Errorf("want the two gates after the failure marked not reached:\n%s", summary)
	}
}

func render(all, selected []gates.Gate, timings []gates.Timing) string {
	var out strings.Builder

	gates.RenderSummary(&out, all, selected, timings)

	return out.String()
}
