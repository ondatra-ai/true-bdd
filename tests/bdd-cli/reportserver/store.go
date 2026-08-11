package reportserver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// The verdict words, aliased so the rest of the package does not reach
// into runner for three strings.
const (
	runnerVerdictPass = runner.VerdictPass
	runnerVerdictFail = runner.VerdictFail
	runnerVerdictSkip = runner.VerdictSkip
)

// DefaultInterval is how stale a snapshot may get before a read
// refreshes it.
const DefaultInterval = 15 * time.Second

// sealed is a fixture that can never change again, with the stat of the
// record that sealed it.
type sealed struct {
	fixture *reporter.Fixture
	modTime time.Time
	size    int64
}

// Store holds every run in memory and refreshes them lazily.
//
// Refresh is pull-based rather than a background ticker: nobody needs a
// fresh scan when no browser is open, and a ticker would burn CPU for
// the life of the process and need shutdown plumbing to stop. A read
// older than the interval does the scan itself.
type Store struct {
	repoRoot string
	runsDir  string
	interval time.Duration
	now      func() time.Time

	// current is swapped atomically. Readers never take a lock.
	current atomic.Pointer[Snapshot]

	// scanMu makes the refresh single-flight: a burst of requests past
	// the interval produces one scan, not one per request.
	scanMu sync.Mutex
	// cache holds sealed fixtures across scans, keyed "<run>/<fixture>".
	// Rebuilt from what each scan sees, so a deleted session evicts
	// itself and the map cannot grow without bound.
	cache map[string]sealed
}

// NewStore builds a store over a runs directory. A relative runsDir
// resolves against the repo root, so the working directory never changes
// the answer.
func NewStore(runsDir string, interval time.Duration) (*Store, error) {
	repoRoot, err := runner.FindRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("find repo root: %w", err)
	}

	if runsDir == "" {
		runsDir = reporter.DefaultRunsDir
	}

	if !filepath.IsAbs(runsDir) {
		runsDir = filepath.Join(repoRoot, runsDir)
	}

	if interval <= 0 {
		interval = DefaultInterval
	}

	return &Store{
		repoRoot: repoRoot,
		runsDir:  runsDir,
		interval: interval,
		now:      time.Now,
		cache:    map[string]sealed{},
	}, nil
}

// RunsDir is the directory being watched.
func (s *Store) RunsDir() string { return s.runsDir }

// Snapshot returns the current view, refreshing it first when the last
// scan is older than the interval.
func (s *Store) Snapshot() (*Snapshot, error) {
	current := s.current.Load()
	if current != nil && s.now().Sub(current.ScannedAt) < s.interval {
		return current, nil
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	// Re-check: another goroutine may have scanned while we waited.
	current = s.current.Load()
	if current != nil && s.now().Sub(current.ScannedAt) < s.interval {
		return current, nil
	}

	return s.refresh(current)
}

// Refresh forces a scan regardless of the interval.
func (s *Store) Refresh() (*Snapshot, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	return s.refresh(s.current.Load())
}

// refresh rebuilds the snapshot. Caller holds scanMu.
func (s *Store) refresh(previous *Snapshot) (*Snapshot, error) {
	names, err := reporter.ListSessions(s.runsDir)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	fresh := make(map[string]sealed, len(s.cache))
	runs := make([]*Run, 0, len(names))

	for _, name := range names {
		run, loadErr := s.loadRun(name, fresh)
		if loadErr != nil {
			return nil, loadErr
		}

		if run != nil {
			runs = append(runs, run)
		}
	}

	s.cache = fresh

	version := int64(1)
	scannedAt := s.now()

	if previous != nil {
		version = previous.Version
		// Only a real content change moves the version — a client uses it
		// to decide whether to re-render, and a version that ticked every
		// scan would throw away scroll position for nothing.
		if changed(previous, runs) {
			version++
		}
	}

	snapshot := newSnapshot(version, scannedAt, runs)
	s.current.Store(snapshot)

	return snapshot, nil
}

// loadRun assembles one session, reusing sealed fixtures. Returns nil for
// a session directory holding no reportable fixture.
func (s *Store) loadRun(name string, fresh map[string]sealed) (*Run, error) {
	dir := filepath.Join(s.runsDir, name)

	// A session that vanished between the listing and here is not an
	// error; the next scan simply will not show it. The read error is
	// deliberately dropped rather than propagated.
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, nil //nolint:nilnil,nilerr // absent session, not a failure
	}

	session := &reporter.Session{Name: name, Dir: dir}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fixtureDir := filepath.Join(dir, entry.Name())
		if !reporter.IsFixtureDir(fixtureDir) {
			continue
		}

		fixture, loadErr := s.loadFixture(name, entry.Name(), fixtureDir, fresh)
		if loadErr != nil {
			return nil, loadErr
		}

		session.Fixtures = append(session.Fixtures, fixture)
	}

	if len(session.Fixtures) == 0 {
		return nil, nil //nolint:nilnil // empty session, nothing to report
	}

	return newRun(session), nil
}

// loadFixture returns a cached fixture when its harness record is
// unchanged, and parses it otherwise.
//
// The record's presence is an exact seal, not a heuristic: it is written
// from the fixture's last t.Cleanup, strictly after everything else the
// run touches. Its mtime and size are carried too, so a re-run that
// reuses a session directory still invalidates.
func (s *Store) loadFixture(
	runID, name, dir string,
	fresh map[string]sealed,
) (*reporter.Fixture, error) {
	key := runID + "/" + name

	info, err := os.Stat(reporter.HarnessRecordPath(dir))
	if err == nil {
		cached, ok := s.cache[key]
		if ok && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
			fresh[key] = cached

			return cached.fixture, nil
		}
	}

	fixture, err := reporter.LoadFixtureDir(dir, name, s.repoRoot)
	if err != nil {
		return nil, fmt.Errorf("load fixture %s: %w", key, err)
	}

	// Only a sealed fixture is cached. One still in flight must be
	// re-read on every scan — that is the run the user is watching.
	if info != nil && fixture.Record != nil {
		fresh[key] = sealed{fixture: fixture, modTime: info.ModTime(), size: info.Size()}
	}

	return fixture, nil
}

// changed reports whether a scan found anything a client would re-render
// for: a run added or removed, or a run's fixture set or outcome moved.
func changed(previous *Snapshot, runs []*Run) bool {
	if len(previous.Runs) != len(runs) {
		return true
	}

	for index, run := range runs {
		old := previous.Runs[index]

		if old.ID != run.ID || old.Complete != run.Complete {
			return true
		}

		if old.Totals != run.Totals {
			return true
		}

		if len(old.Tests) != len(run.Tests) {
			return true
		}
	}

	return false
}
