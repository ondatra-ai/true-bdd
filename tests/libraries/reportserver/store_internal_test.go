package reportserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// newTestStore builds a store over a synthetic runs directory with a
// clock the test drives, so interval behaviour is tested without
// sleeping.
func newTestStore(t *testing.T, runsDir string) (*Store, *time.Time) {
	t.Helper()

	store, err := NewStore(runsDir, time.Minute)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	clock := time.Now()
	store.now = func() time.Time { return clock }

	return store, &clock
}

// writeFixture materialises one fixture run. A sealed fixture gets a
// harness record; an unsealed one gets only an engine log, exactly as a
// run still in flight looks on disk.
func writeFixture(t *testing.T, runsDir, run, name string, sealed bool) {
	t.Helper()

	dir := filepath.Join(runsDir, run, name)

	err := os.MkdirAll(filepath.Join(dir, "tmp"), 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// One dispatch/return pair, so the fixture has a turn to count.
	log := `{"time":"2026-08-10T23:33:51.616772+02:00","level":"INFO","msg":"Dispatching AI turn",` +
		`"turn":1,"label":"t1","role":"prompt","cli":"claude","model":"opus"}` + "\n" +
		`{"time":"2026-08-10T23:34:11.841105+02:00","level":"INFO","msg":"AI turn returned",` +
		`"turn":1,"label":"t1","role":"prompt","cli":"claude","model":"opus","duration_ms":20224}` + "\n"

	err = os.WriteFile(filepath.Join(dir, "tmp", "true-bdd.log.json"), []byte(log), 0o644)
	if err != nil {
		t.Fatalf("write engine log: %v", err)
	}

	if !sealed {
		return
	}

	writeRecord(t, dir, name, runner.VerdictPass)
}

// writeRecord writes a fixture's harness record — the seal.
func writeRecord(t *testing.T, dir, name, verdict string) {
	t.Helper()

	logDir := filepath.Join(dir, runner.SpawnLogDir)

	err := os.MkdirAll(logDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	blob, err := json.Marshal(runner.HarnessRecord{
		Schema: 2, Fixture: name, Verdict: verdict, WallMs: 1000,
	})
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	err = os.WriteFile(filepath.Join(logDir, runner.HarnessRecordFile), blob, 0o644)
	if err != nil {
		t.Fatalf("write record: %v", err)
	}
}

// TestSealedFixturesAreNotReparsed pins the cache key that makes a 15s
// refresh affordable: a rescan must hand back the same pointer for a
// sealed fixture, not re-parse its engine log.
func TestSealedFixturesAreNotReparsed(t *testing.T) {
	runsDir := t.TempDir()
	writeFixture(t, runsDir, "2026-01-01_00-00-00", "sealed-fx", true)
	writeFixture(t, runsDir, "2026-01-01_00-00-00", "live-fx", false)

	store, _ := newTestStore(t, runsDir)

	first, err := store.Refresh()
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}

	second, err := store.Refresh()
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}

	run, ok := first.Run("2026-01-01_00-00-00")
	if !ok {
		t.Fatal("run missing from first snapshot")
	}

	rerun, _ := second.Run("2026-01-01_00-00-00")

	sealedBefore, _ := run.Test("sealed-fx")
	sealedAfter, _ := rerun.Test("sealed-fx")

	if sealedBefore != sealedAfter {
		t.Error("sealed fixture was re-parsed; the harness record should have cached it")
	}

	liveBefore, _ := run.Test("live-fx")
	liveAfter, _ := rerun.Test("live-fx")

	if liveBefore == liveAfter {
		t.Error("unsealed fixture was cached; a run in flight must be re-read every scan")
	}

	// A run holding one unsealed fixture is not complete.
	if rerun.Complete {
		t.Error("run reported complete while a fixture has no harness record")
	}
}

// TestVersionMovesOnlyOnRealChange pins that a client polling every few
// seconds is not told to re-render by a scan that found nothing.
func TestVersionMovesOnlyOnRealChange(t *testing.T) {
	runsDir := t.TempDir()
	writeFixture(t, runsDir, "2026-01-01_00-00-00", "fx", true)

	store, _ := newTestStore(t, runsDir)

	first, err := store.Refresh()
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}

	quiet, err := store.Refresh()
	if err != nil {
		t.Fatalf("quiet scan: %v", err)
	}

	if quiet.Version != first.Version {
		t.Errorf("version moved %d -> %d with no change on disk",
			first.Version, quiet.Version)
	}

	writeFixture(t, runsDir, "2026-01-02_00-00-00", "fx", true)

	grown, err := store.Refresh()
	if err != nil {
		t.Fatalf("grown scan: %v", err)
	}

	if grown.Version == first.Version {
		t.Errorf("version stayed %d after a new run appeared", grown.Version)
	}

	if len(grown.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(grown.Runs))
	}
}

// TestSnapshotHonoursInterval pins that reads inside the interval are
// free — no directory scan at all — and that crossing it refreshes.
func TestSnapshotHonoursInterval(t *testing.T) {
	runsDir := t.TempDir()
	writeFixture(t, runsDir, "2026-01-01_00-00-00", "fx", true)

	store, clock := newTestStore(t, runsDir)

	first, err := store.Snapshot()
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	// A new run lands, but the interval has not elapsed.
	writeFixture(t, runsDir, "2026-01-02_00-00-00", "fx", true)

	cached, err := store.Snapshot()
	if err != nil {
		t.Fatalf("cached: %v", err)
	}

	if cached != first {
		t.Error("snapshot rescanned inside the interval")
	}

	*clock = clock.Add(2 * time.Minute)

	refreshed, err := store.Snapshot()
	if err != nil {
		t.Fatalf("refreshed: %v", err)
	}

	if len(refreshed.Runs) != 2 {
		t.Errorf("runs = %d after the interval elapsed, want 2", len(refreshed.Runs))
	}
}

// TestNeighboursWalkRunsInOrder pins the prev/next navigation contract,
// including that the ends report empty rather than wrapping.
func TestNeighboursWalkRunsInOrder(t *testing.T) {
	runsDir := t.TempDir()
	for _, id := range []string{"2026-01-01_00-00-00", "2026-01-02_00-00-00", "2026-01-03_00-00-00"} {
		writeFixture(t, runsDir, id, "fx", true)
	}

	store, _ := newTestStore(t, runsDir)

	snapshot, err := store.Refresh()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	prev, next := snapshot.Neighbours("2026-01-02_00-00-00")
	if prev != "2026-01-01_00-00-00" || next != "2026-01-03_00-00-00" {
		t.Errorf("middle neighbours = (%q, %q)", prev, next)
	}

	prev, _ = snapshot.Neighbours("2026-01-01_00-00-00")
	if prev != "" {
		t.Errorf("oldest run has prev %q, want none", prev)
	}

	_, next = snapshot.Neighbours("2026-01-03_00-00-00")
	if next != "" {
		t.Errorf("newest run has next %q, want none", next)
	}
}

// The store assembles its own Session (not reporter.LoadSession) to reuse
// sealed fixtures across scans; that divergence already cost one silently
// empty mode column, so this pins the store's own path.
func TestStoreReadsSessionMeta(t *testing.T) {
	t.Parallel()

	runsDir := t.TempDir()
	writeFixture(t, runsDir, "2026-08-15_08-19-12", "fx", true)

	err := runner.WriteSessionMeta(filepath.Join(runsDir, "2026-08-15_08-19-12"),
		runner.ProxyModeReplay, []string{"fx", "not-started-yet"})
	if err != nil {
		t.Fatalf("write session meta: %v", err)
	}

	store, _ := newTestStore(t, runsDir)

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	run, ok := snapshot.Run("2026-08-15_08-19-12")
	if !ok {
		t.Fatal("run not found")
	}

	if run.Mode != runner.ProxyModeReplay {
		t.Fatalf("mode = %q, want %q", run.Mode, runner.ProxyModeReplay)
	}

	// Planned counts what the invocation set out to run, not what has
	// finished — the whole reason it is recorded up front.
	if run.Planned != 2 {
		t.Fatalf("planned = %d, want 2", run.Planned)
	}
}

// A session written before the record existed must read as unknown, not
// as live: a replayed run and a live one prove different things.
func TestStoreToleratesMissingSessionMeta(t *testing.T) {
	t.Parallel()

	runsDir := t.TempDir()
	writeFixture(t, runsDir, "2026-08-10_10-00-00", "fx", true)

	store, _ := newTestStore(t, runsDir)

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	run, _ := snapshot.Run("2026-08-10_10-00-00")
	if run.Mode != "" {
		t.Fatalf("mode = %q, want empty (unknown)", run.Mode)
	}

	// With nothing recorded, the only honest denominator is what is here.
	if run.Planned != 1 {
		t.Fatalf("planned = %d, want 1 (fallback to observed)", run.Planned)
	}
}
