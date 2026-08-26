package merge_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

const (
	quick = time.Minute
	long  = 40 * time.Minute
	// The phase name and outcome the fixtures reuse.
	mergePhase   = "merge"
	mergeOutcome = "merged"
)

// The two conditions are alternatives. Under a mandate a short clean run is
// the one case that has nothing to teach worth ~9 minutes of model time.
func TestPostmortemGate(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		automatic bool
		total     time.Duration
		want      bool
	}{
		{"a run that needed a human", false, quick, true},
		{"a long automatic run", true, long, true},
		{"a long run that needed a human", false, long, true},
		{"a short clean automatic run", true, quick, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			worth, why := merge.WorthAPostmortem(testCase.automatic, testCase.total)
			if worth != testCase.want {
				t.Fatalf("worth=%v, want %v (%s)", worth, testCase.want, why)
			}

			if why == "" {
				t.Error("the decision came with no reason to print")
			}
		})
	}
}

// The skip has to be visible: a postmortem that is silently absent reads
// exactly like one that ran and found nothing.
func TestPostmortemGateSaysWhyItSkipped(t *testing.T) {
	t.Parallel()

	_, why := merge.WorthAPostmortem(true, quick)
	if !strings.Contains(why, merge.PostmortemFloor.String()) {
		t.Errorf("the skip reason does not name the floor: %q", why)
	}
}

func TestPostmortemGateFloorIsFifteenMinutes(t *testing.T) {
	t.Parallel()

	if merge.PostmortemFloor != 15*time.Minute {
		t.Errorf("floor is %s, want 15m", merge.PostmortemFloor)
	}
}

func TestPostmortemPromptCarriesThePlaceholders(t *testing.T) {
	t.Parallel()

	prompt := merge.PostmortemPrompt()
	for _, placeholder := range []string{"{transcript}", "{timings}"} {
		if !strings.Contains(prompt, placeholder) {
			t.Errorf("the prompt has no %s to substitute", placeholder)
		}
	}
}

func TestPostmortemPromptSubstitutesTheTimings(t *testing.T) {
	t.Parallel()

	rendered := merge.RenderPostmortemPrompt("the turns", "| commit | 12.0 |")

	if !strings.Contains(rendered, "| commit | 12.0 |") {
		t.Error("the timing table did not reach the prompt")
	}

	if strings.Contains(rendered, "{timings}") || strings.Contains(rendered, "{transcript}") {
		t.Error("a placeholder survived rendering, and would read as an instruction")
	}
}

// An unmeasured run must say so in words. A raw {timings} left in the prompt
// would read as an instruction to the model.
func TestPostmortemPromptNamesAnUnmeasuredRun(t *testing.T) {
	t.Parallel()

	rendered := merge.RenderPostmortemPrompt("the turns", "")

	if strings.Contains(rendered, "{timings}") {
		t.Fatal("an empty table left the placeholder in the prompt")
	}

	if !strings.Contains(rendered, "no timing record") {
		t.Error("an empty table is not reported as one")
	}
}

func TestPostmortemTimingReportRendersEveryPhase(t *testing.T) {
	t.Parallel()

	table := merge.TimingReport{
		PR:            99,
		CommitSeconds: 310,
		TotalSeconds:  1240,
		Phases: []merge.Phase{
			{Name: "request_review", Round: 1, Seconds: 780, Outcome: "requested"},
			{Name: mergePhase, Seconds: 42.5, Outcome: mergeOutcome},
		},
	}.Render()

	for _, want := range []string{"request_review", "780.0", mergePhase, "42.5", mergeOutcome, "1240.0"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table does not carry %q:\n%s", want, table)
		}
	}

	if !strings.Contains(table, "310") {
		t.Errorf("the commit that produced the PR is missing from the table:\n%s", table)
	}
}

func TestPostmortemLedgerSumsTheSecondsColumn(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "timings.tsv")

	err := os.WriteFile(path,
		[]byte("gates\t142\nscan-recordings\t3\ncommit-msg (claude)\t28\nmalformed\n"), 0o600)
	if err != nil {
		t.Fatalf("writing the ledger: %v", err)
	}

	if total := merge.ReadLedger(path); total != 173*time.Second {
		t.Errorf("summed %s, want 173s", total)
	}
}

// Timing must never be able to stop a merge, so an absent ledger is zero.
func TestPostmortemLedgerIsZeroWhenAbsent(t *testing.T) {
	t.Parallel()

	if total := merge.ReadLedger(filepath.Join(t.TempDir(), "nothing.tsv")); total != 0 {
		t.Errorf("a missing ledger summed to %s", total)
	}
}

// The merge loop writes the record and the standalone entry point reads it
// back, so the JSON is the contract between them.
func TestPostmortemTimingRecordRoundTrips(t *testing.T) {
	t.Parallel()

	original := merge.TimingReport{
		PR: 99, StartedAt: "2026-08-26T20:00:00Z", CommitSeconds: 310, TotalSeconds: 1240,
		Phases: []merge.Phase{{Name: "merge", Seconds: 42.5, Outcome: "merged"}},
	}

	encoded, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}

	var decoded merge.TimingReport

	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if decoded.Render() != original.Render() {
		t.Errorf("the record does not survive the round trip:\n%s\n%s",
			original.Render(), decoded.Render())
	}
}

// Verification 6: the skip has to reach stdout. Not parallel — it swaps
// os.Stdout for a pipe to read what the run printed.
func TestPostmortemSkipIsPrinted(t *testing.T) {
	printed := merge.SkipPostmortem(time.Minute)

	if !strings.Contains(printed, "postmortem") {
		t.Errorf("the skip did not announce which step it was: %q", printed)
	}

	if !strings.Contains(printed, "skipped") {
		t.Errorf("a skipped postmortem was silently absent: %q", printed)
	}

	if !strings.Contains(printed, merge.PostmortemFloor.String()) {
		t.Errorf("the printed skip does not name the floor: %q", printed)
	}
}
