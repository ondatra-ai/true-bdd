// Package reportserver serves the BDD run report over HTTP.
//
// It holds every session under tmp/test_run in memory, refreshes them on
// a interval, and answers questions the old static report could not: what
// ran across all sessions, how two runs differ test by test, and how one
// test's turns line up between them.
//
// The loading is reporter's; this package owns the caching, the JSON
// contract, the diffing and the UI.
package reportserver

import (
	"sort"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/reporter"
)

// Totals is a run's roll-up, summed straight from what the harness and
// the engine measured. No phase model, no attribution.
type Totals struct {
	Fixtures int
	Passed   int
	Failed   int
	Skipped  int
	Turns    int
	Tokens   int
	Wall     time.Duration
	CostUSD  float64
	// JudgeCostUSD is billed separately from CostUSD: the harness spending
	// to grade, not the engine spending to work — mixed, an expensive
	// fixture becomes indistinguishable from an expensive verdict.
	JudgeCostUSD float64
}

// Run is one session, loaded.
type Run struct {
	ID  string
	Dir string
	// Mode is how this session reached the models: live, record or
	// replay. Empty means the session predates the record — unknown, not
	// live.
	Mode string
	// Planned is how many fixtures the invocation set out to run — the
	// honest denominator for "passed", since fixtures on disk are only
	// the finished ones.
	Planned int
	// Complete is true when every fixture has a harness record (see
	// reporter.HarnessRecordPath) — a complete run can never change again,
	// which is what makes it safe to cache forever.
	Complete bool
	Tests    []*reporter.Fixture
	Totals   Totals

	byName map[string]*reporter.Fixture
}

// Test returns one fixture by name.
func (r *Run) Test(name string) (*reporter.Fixture, bool) {
	fixture, ok := r.byName[name]

	return fixture, ok
}

// newRun indexes a session's fixtures and sums its totals.
func newRun(session *reporter.Session) *Run {
	run := &Run{
		ID:       session.Name,
		Dir:      session.Dir,
		Mode:     session.Mode,
		Planned:  len(session.Planned),
		Tests:    session.Fixtures,
		Complete: true,
		byName:   make(map[string]*reporter.Fixture, len(session.Fixtures)),
	}

	// A session with no planned-record (predating the harness writing one)
	// falls back to what it can see — never smaller than the truth, and it
	// keeps those runs renderable.
	if run.Planned == 0 {
		run.Planned = len(session.Fixtures)
	}

	for _, fixture := range session.Fixtures {
		run.byName[fixture.Name] = fixture
		run.Totals.add(fixture)

		if fixture.Record == nil {
			run.Complete = false
		}
	}

	return run
}

// add folds one fixture into the roll-up.
func (t *Totals) add(fixture *reporter.Fixture) {
	t.Fixtures++
	t.Turns += len(fixture.Turns)
	t.Tokens += fixture.Tokens
	t.Wall += fixture.Wall
	t.CostUSD += fixture.Cost

	if fixture.Judge != nil {
		t.JudgeCostUSD += fixture.Judge.CostUSD
	}

	switch fixture.Verdict {
	case runnerVerdictPass:
		t.Passed++
	case runnerVerdictFail:
		t.Failed++
	case runnerVerdictSkip:
		t.Skipped++
	}
}

// Snapshot is one consistent view of every run on disk, immutable once
// published: readers hold the pointer for a whole request, so a refresh
// mid-request can never tear a response.
type Snapshot struct {
	// Version increments only when the content actually changed, so a
	// polling client can hold its scroll position through a scan that
	// found nothing new.
	Version   int64
	ScannedAt time.Time
	// Runs is oldest first — session names are sortable timestamps.
	Runs []*Run

	byID map[string]*Run
}

// Run returns one run by id.
func (s *Snapshot) Run(id string) (*Run, bool) {
	run, ok := s.byID[id]

	return run, ok
}

// Neighbours returns the run ids either side of id, for prev/next
// navigation. Empty strings at the ends.
func (s *Snapshot) Neighbours(id string) (string, string) {
	for index, run := range s.Runs {
		if run.ID != id {
			continue
		}

		prev, next := "", ""

		if index > 0 {
			prev = s.Runs[index-1].ID
		}

		if index+1 < len(s.Runs) {
			next = s.Runs[index+1].ID
		}

		return prev, next
	}

	return "", ""
}

// newSnapshot indexes runs and fixes their order.
func newSnapshot(version int64, at time.Time, runs []*Run) *Snapshot {
	sort.Slice(runs, func(i, j int) bool { return runs[i].ID < runs[j].ID })

	snapshot := &Snapshot{
		Version:   version,
		ScannedAt: at,
		Runs:      runs,
		byID:      make(map[string]*Run, len(runs)),
	}

	for _, run := range runs {
		snapshot.byID[run.ID] = run
	}

	return snapshot
}
