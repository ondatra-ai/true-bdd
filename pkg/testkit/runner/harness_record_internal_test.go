package runner

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/fstree"
)

// TestRecordSurvivesGoexit pins the reason Finish is wired through
// t.Cleanup, not a statement at the end of runFixture: t.Fatalf leaves
// via runtime.Goexit, skipping later statements — Cleanup still runs.
func TestRecordSurvivesGoexit(t *testing.T) {
	session := t.TempDir()
	rec := NewHarnessRecorder(session, "fx", ProxyModeLive, nil)

	done := make(chan struct{})

	go func() {
		defer close(done)
		defer rec.Finish(true, false)

		runtime.Goexit()
	}()

	<-done

	path := filepath.Join(RunDir(session, "fx"), SpawnLogDir, HarnessRecordFile)

	blob, err := disk.Read(path)
	if err != nil {
		t.Fatalf("record not written after Goexit: %v", err)
	}

	var record HarnessRecord

	err = json.Unmarshal(blob, &record)
	if err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}

	if record.Verdict != VerdictFail {
		t.Errorf("verdict = %q, want %q", record.Verdict, VerdictFail)
	}

	if record.Schema != harnessSchema {
		t.Errorf("schema = %d, want %d", record.Schema, harnessSchema)
	}
}

// TestPostRunWriteIsNotInDiff pins the placement invariant: the record
// lands inside the tree the judge grades, safe only because both
// snapshots are already taken — moving the write inside Execute would break this.
func TestPostRunWriteIsNotInDiff(t *testing.T) {
	tmpDir := t.TempDir()

	err := disk.Write(filepath.Join(tmpDir, "src.txt"), []byte("x"), disk.Shared)
	if err != nil {
		t.Fatalf("seed tree: %v", err)
	}

	before, err := fstree.Snapshot(tmpDir)
	if err != nil {
		t.Fatalf("snapshot before: %v", err)
	}

	after, err := fstree.Snapshot(tmpDir)
	if err != nil {
		t.Fatalf("snapshot after: %v", err)
	}

	// Exactly the ordering Execute + the t.Cleanup produce. Every file
	// Finish writes goes through one of these two calls, so covering both
	// keeps the judge transcript and manifest snapshot out of the graded diff.
	logDir := filepath.Join(tmpDir, SpawnLogDir)

	writeSidecars(logDir, map[string]string{
		JudgeSystemFile:      "system",
		JudgeUserFile:        "user",
		JudgeResponseFile:    VerdictPass,
		ManifestSnapshotFile: "{}",
	})
	writeHarnessRecord(logDir, HarnessRecord{Fixture: "fx"})

	diff := fstree.Diff(before, after)
	if len(diff) != 0 {
		t.Errorf("post-run harness writes polluted the judge diff: %+v", diff)
	}
}

// TestSidecarsRecordOnlyWhatLanded pins that HarnessRecord.Artifacts
// advertises only files a reader can actually open — the report uses
// this list to tell "judge never reached" from "predates the transcript".
func TestSidecarsRecordOnlyWhatLanded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), SpawnLogDir)

	written := writeSidecars(dir, map[string]string{
		JudgeUserFile:     "the prompt",
		JudgeResponseFile: VerdictPass,
	})

	want := []string{JudgeResponseFile, JudgeUserFile} // sorted
	if len(written) != len(want) {
		t.Fatalf("wrote %v, want %v", written, want)
	}

	for i, name := range want {
		if written[i] != name {
			t.Errorf("artifact[%d] = %q, want %q", i, written[i], name)
		}

		_, statErr := os.Stat(filepath.Join(dir, name))
		if statErr != nil {
			t.Errorf("advertised %q but it is not readable: %v", name, statErr)
		}
	}

	// A judge that was never reached contributes no empty files.
	got := writeSidecars(dir, map[string]string{})
	if got != nil {
		t.Errorf("empty sidecar set wrote %v, want nil", got)
	}
}

// TestUsageOutsideJudgeWindowIsNotBilled pins the fix for the pairing
// defect: a fixture that died before reaching its judge must claim
// nothing rather than inherit a neighbour's cost.
func TestUsageOutsideJudgeWindowIsNotBilled(t *testing.T) {
	sink := &UsageSink{}
	base := time.Now()

	sink.add(usageRecord(base, 0.5, 100))
	sink.add(usageRecord(base.Add(2*time.Second), 0.25, 40))

	cost, tokens := sink.Between(base.Add(time.Second), base.Add(3*time.Second))
	if cost != 0.25 {
		t.Errorf("cost = %v, want 0.25 (the record before the window must not be billed)", cost)
	}

	if tokens["input_tokens"] != 40 {
		t.Errorf("tokens = %v, want 40", tokens["input_tokens"])
	}

	// A fixture that never reached its judge has a zero window.
	rec := NewHarnessRecorder(t.TempDir(), "fx", ProxyModeLive, sink)
	rec.billJudge()

	if rec.record.Judge.CostUSD != 0 {
		t.Errorf("cost = %v on an unstamped window, want 0", rec.record.Judge.CostUSD)
	}
}

// TestSinkReadsClaudeUsageFields pins the coupling to logTurnUsage in
// src/adapters/ai/claude_provider.go. The CLI's usage block is decoded
// into map[string]any, so the counters reach slog as float64.
func TestSinkReadsClaudeUsageFields(t *testing.T) {
	sink := &UsageSink{}
	stamp := time.Now()

	record := slog.NewRecord(stamp, slog.LevelInfo, msgAITurnUsage, 0)
	record.Add("cli", "claude", "cost_usd", 0.4174)

	for index, key := range usageTokenKeys() {
		record.Add(key, float64(index+1))
	}

	sink.add(record)

	cost, tokens := sink.Between(stamp.Add(-time.Second), stamp.Add(time.Second))
	if cost != 0.4174 {
		t.Errorf("cost = %v, want 0.4174", cost)
	}

	if len(tokens) != len(usageTokenKeys()) {
		t.Fatalf("token kinds = %d, want %d — a counter name drifted from the provider",
			len(tokens), len(usageTokenKeys()))
	}

	total := 0
	for _, count := range tokens {
		total += count
	}

	if total != 1+2+3+4 {
		t.Errorf("token total = %d, want 10", total)
	}
}

// TestVerdictWord pins PASS/FAIL/SKIP parity with what `go test -v`
// prints for a subtest, which is the report's existing vocabulary.
func TestVerdictWord(t *testing.T) {
	cases := []struct {
		failed  bool
		skipped bool
		want    string
	}{
		{false, false, VerdictPass},
		{true, false, VerdictFail},
		{false, true, VerdictSkip},
		// A skipped test is not failed; skip wins if both are somehow set.
		{true, true, VerdictSkip},
	}

	for _, testCase := range cases {
		got := verdictWord(testCase.failed, testCase.skipped)
		if got != testCase.want {
			t.Errorf("verdictWord(%v, %v) = %q, want %q",
				testCase.failed, testCase.skipped, got, testCase.want)
		}
	}
}

// TestDiffRecordsCarrySizes pins that both ends of a modification are
// recorded, so the report can say what a file changed from as well as to.
func TestDiffRecordsCarrySizes(t *testing.T) {
	records := diffRecords([]FileChange{
		{Path: "a.ts", Kind: "modified", Before: []byte("ab"), After: []byte("abcd")},
	})

	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}

	if records[0].BytesBefore != 2 || records[0].BytesAfter != 4 {
		t.Errorf("sizes = %d/%d, want 2/4", records[0].BytesBefore, records[0].BytesAfter)
	}
}

// usageRecord builds one "AI turn usage" record with a single counter.
func usageRecord(at time.Time, cost float64, tokens int) slog.Record {
	record := slog.NewRecord(at, slog.LevelInfo, msgAITurnUsage, 0)
	record.Add("cost_usd", cost, "input_tokens", float64(tokens))

	return record
}
