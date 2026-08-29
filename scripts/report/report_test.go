package report_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

const run = "0e1f7b2f"

// line builds one log record. Times are seconds into the run, which is all the
// folding rules read them for.
func line(second int, level, msg, extra string) string {
	stamp := "2026-08-29T08:00:" + pad(second) + ".000000Z"

	return `{"time":"` + stamp + `","level":"` + level + `","msg":"` + msg +
		`","tool":"commit","run":"` + run + `"` + extra + `}`
}

func pad(second int) string {
	if second < 10 {
		return "0" + string(rune('0'+second))
	}

	return string(rune('0'+second/10)) + string(rune('0'+second%10))
}

const (
	start = `,"tree":"start"`
	end   = `,"tree":"end"`
)

// fold writes the lines to a file and folds them, the way every caller does.
func fold(t *testing.T, lines ...string) *report.Run {
	t.Helper()

	path := filepath.Join(t.TempDir(), "task.log.json")

	err := disk.Write(path, []byte(strings.Join(lines, "\n")+"\n"), disk.Shared)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	folded, err := report.Fold(path, run)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}

	return folded
}

func TestNestedMarkersBecomeATree(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		line(1, "INFO", "alint", `,"duration_ms":250`),
		line(2, "INFO", "select", start),
		line(4, "INFO", "select", end),
		line(6, "INFO", "gates", end),
		line(6, "INFO", "commit", start),
		line(9, "INFO", "commit", end),
	)

	if len(folded.Nodes) != 2 {
		t.Fatalf("%d top-level nodes, want 2", len(folded.Nodes))
	}

	gates := folded.Nodes[0]
	if gates.Name != "gates" || len(gates.Children) != 2 {
		t.Fatalf("gates = %q with %d children, want 2", gates.Name, len(gates.Children))
	}

	if got := gates.Duration.Seconds(); got != 6 {
		t.Errorf("gates took %vs, want 6", got)
	}

	if got := gates.Children[0].Duration.Milliseconds(); got != 250 {
		t.Errorf("the leaf took %dms, want its own 250", got)
	}

	if got := gates.Children[1].Duration.Seconds(); got != 2 {
		t.Errorf("the nested node took %vs, want 2", got)
	}

	if folded.Status != report.StatusDone {
		t.Errorf("run = %s, want done", folded.Status)
	}
}

// dief logs ERROR and calls os.Exit, which skips every defer: the stack is
// left open, and the ERROR is what separates that from a kill.
func TestAnErrorFailsEveryOpenNode(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		line(1, "INFO", "run the pipeline", start),
		line(3, "ERROR", "STOPPED: the gates are red", ""),
	)

	gates := folded.Nodes[0]
	if gates.Status != report.StatusFailed {
		t.Errorf("the open outer node = %s, want failed", gates.Status)
	}

	if got := gates.Children[0].Status; got != report.StatusFailed {
		t.Errorf("the open inner node = %s, want failed", got)
	}

	if folded.Status != report.StatusFailed {
		t.Errorf("run = %s, want failed", folded.Status)
	}
}

// SIGKILL skips even the ERROR, so silence with an open node is all there is.
func TestAnOpenNodeWithNoErrorReadsAsKilled(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		line(1, "INFO", "alint", `,"duration_ms":250`),
	)

	if got := folded.Nodes[0].Status; got != report.StatusKilled {
		t.Errorf("the open node = %s, want killed", got)
	}
}

// A WARN marks where it happened and nothing above it: merge's postmortem
// warns and returns, and the stages before it did finish.
func TestAWarnMarksOnlyItsOwnNode(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "merge", start),
		line(1, "INFO", "merge", end),
		line(1, "INFO", "postmortem", start),
		line(2, "WARN", "the postmortem turn failed", ""),
		line(3, "INFO", "postmortem", end),
	)

	if got := folded.Nodes[0].Status; got != report.StatusDone {
		t.Errorf("the earlier node = %s, want done", got)
	}

	if got := folded.Nodes[1].Status; got != report.StatusWarned {
		t.Errorf("the warning's node = %s, want warned", got)
	}

	if folded.Status != report.StatusWarned {
		t.Errorf("run = %s, want warned", folded.Status)
	}
}

// A leaf that failed but was handled must not fail the node that handled it —
// which is the whole reason a node's status comes from its own records.
func TestAFailedLeafDoesNotFailItsNode(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "postmortem", start),
		line(2, "INFO", "ai turn", `,"duration_ms":1500,"status":"failed"`),
		line(3, "INFO", "postmortem", end),
	)

	node := folded.Nodes[0]
	if node.Status != report.StatusDone {
		t.Errorf("the node = %s, want done", node.Status)
	}

	if got := node.Children[0].Status; got != report.StatusFailed {
		t.Errorf("the leaf = %s, want failed", got)
	}

	if folded.Status != report.StatusFailed {
		t.Errorf("run = %s, want the worst thing that happened", folded.Status)
	}
}

func TestASwitchedOffStageReadsAsSkipped(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "doc universe", start+`,"skipped":true`),
		line(0, "INFO", "doc universe", end),
	)

	if got := folded.Nodes[0].Status; got != report.StatusSkipped {
		t.Errorf("the switched-off node = %s, want skipped", got)
	}
}

// usage() stops before any node exists, so the refusal has nowhere else to go.
func TestRecordsBeforeTheFirstMarkerArePreflight(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "ERROR", "not inside a git repository", ""),
	)

	if len(folded.Nodes) != 1 || folded.Nodes[0].Name != "preflight" {
		t.Fatalf("nodes = %v, want one preflight row", folded.Nodes)
	}

	if folded.Status != report.StatusFailed {
		t.Errorf("run = %s, want failed", folded.Status)
	}
}

// merge's fix agents spawn gates as a separate process into the same file:
// without the filter, its markers interleave and the stack is corrupt.
func TestOnlyTheNamedRunIsFolded(t *testing.T) {
	t.Parallel()

	other := strings.ReplaceAll(line(1, "INFO", "gate", start), run, "ffffffff")

	folded := fold(t,
		line(0, "INFO", "gates", start),
		other,
		line(2, "INFO", "gates", end),
	)

	if len(folded.Nodes) != 1 || folded.Nodes[0].Status != report.StatusDone {
		t.Fatalf("the other run leaked in: %v", folded.Nodes)
	}
}

func TestATornLineIsSkippedRatherThanFatal(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		`{"time":"2026-08-29T08:00:01.0`,
		line(2, "INFO", "gates", end),
	)

	if got := folded.Nodes[0].Status; got != report.StatusDone {
		t.Errorf("the node = %s, want the torn line ignored", got)
	}
}

func TestTableRendersDepthAsANumberAndAbsenceAsADash(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		line(1, "INFO", "alint", `,"duration_ms":250`),
		line(2, "INFO", "gates", end),
		line(2, "INFO", "doc universe", start+`,"skipped":true`),
		line(2, "INFO", "doc universe", end),
		line(2, "INFO", "commit", start),
	)

	table := folded.Table()

	for _, want := range []string{
		"| 1 | gates | done | 2.0s |",
		"| 1.1 | alint | done | 250ms |",
		"| 2 | doc universe | skipped | — |",
		"| 3 | commit | killed | — |",
		"| | **commit** | **killed** | **2.0s** |",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table is missing %q:\n%s", want, table)
		}
	}
}

// gates says nothing structured after its last leaf: a stop there closes no
// node, so the run itself is the only thing left to carry the failure.
func TestAnErrorWithNothingOpenStillFailsTheRun(t *testing.T) {
	t.Parallel()

	folded := fold(t,
		line(0, "INFO", "gates", start),
		line(2, "INFO", "gates", end),
		line(3, "ERROR", "gates failed", ""),
	)

	if got := folded.Nodes[0].Status; got != report.StatusDone {
		t.Errorf("the closed node = %s, want done", got)
	}

	if folded.Status != report.StatusFailed {
		t.Errorf("run = %s, want failed", folded.Status)
	}
}
