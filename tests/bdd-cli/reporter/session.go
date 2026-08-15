// Package reporter loads a true-bdd BDD fixture session off disk and
// turns it into an object graph the report server can serve.
//
// It parses, it does not render. Four sources per fixture:
//
//  1. tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json — the
//     engine's own JSON log. Every AI turn leaves "Dispatching AI turn"
//     (turn/role/cli/model), "AI turn usage" (cost and tokens) and "AI
//     turn returned"/"AI turn failed" (duration_ms). The spans BETWEEN
//     those records are the engine's deterministic work.
//
//  2. tmp/test_run/<session>/<fixture>/bdd-cli-logs/harness.json — the
//     harness's own record. Four things live only here, because the
//     engine cannot see them from inside its own process: the fixture's
//     wall clock, its verdict, the file diff, and what the judge spent.
//
//  3. The fixture's rendered prompt artifacts, for the runner's
//     <startup> marker and for failure blocks that arrived empty.
//
//  4. The fixture manifest — this run's snapshot when the harness wrote
//     one, else the fixture folder in the source tree.
//
// The phase timeline it builds is a gap-free accounting of each
// fixture's wall clock: every slice is tagged deterministic (Go code
// whose duration is a property of the machine) or model (an agent
// deciding how long to take), and the two must sum to the wall clock the
// harness measured. That invariant is pinned by the package's tests.
package reporter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// ErrNoSessions is returned when there is nothing to report on.
var ErrNoSessions = errors.New("no fixture run sessions found — run the BDD suite first")

// DefaultRunsDir is where the BDD harness writes its sessions, relative
// to the repo root.
const DefaultRunsDir = "tmp/test_run"

// engineLogDir / engineLogFile locate the engine's JSON log inside a
// fixture's run directory.
const (
	engineLogDir  = "tmp"
	engineLogFile = "true-bdd.log.json"
)

// EngineLogPath is the engine's own JSON log for one fixture run.
func EngineLogPath(dir string) string {
	return filepath.Join(dir, engineLogDir, engineLogFile)
}

// Session is one run of the BDD suite: a directory under tmp/test_run
// holding one subdirectory per fixture that ran.
type Session struct {
	// Name is the directory name, a sortable "2006-01-02_15-04-05".
	Name string
	// Dir is the absolute path to the session directory.
	Dir string
	// Fixtures is every fixture that left a trace, in directory order.
	Fixtures []*Fixture
	// Mode is the AI-CLI mode the whole invocation ran under — live,
	// record or replay. Empty for a session recorded before the harness
	// wrote a session record, which a reader must show as unknown rather
	// than assume was live.
	Mode string
	// Planned is the fixtures this invocation set out to run, whether or
	// not they have finished. Empty for the same older sessions; a reader
	// then has nothing better than the fixtures it can see.
	Planned []string
}

// ListSessions returns the session directory names under runsDir,
// oldest first — the names are sortable timestamps, so lexical order is
// chronological order.
func ListSessions(runsDir string) ([]string, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNoSessions, runsDir)
	}

	var names []string

	for _, entry := range entries {
		// harness.log.json lives at the session root as a FILE precisely
		// so this scan cannot mistake it for a fixture.
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)

	return names, nil
}

// LoadSession assembles every fixture in one session directory.
func LoadSession(dir, repoRoot string) (*Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	session := &Session{Name: filepath.Base(dir), Dir: dir}

	ApplySessionMeta(session)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fixtureDir := filepath.Join(dir, entry.Name())
		if !IsFixtureDir(fixtureDir) {
			continue
		}

		fixture, loadErr := LoadFixtureDir(fixtureDir, entry.Name(), repoRoot)
		if loadErr != nil {
			return nil, loadErr
		}

		session.Fixtures = append(session.Fixtures, fixture)
	}

	return session, nil
}

// ApplySessionMeta folds the session record — the AI mode and the
// planned fixture list — into a Session.
//
// Exported and separate because a Session is assembled in two places:
// LoadSession here, and the report server's own store, which builds one
// incrementally so it can reuse sealed fixtures across scans. Reading
// the record in only one of them is exactly the drift this function
// exists to prevent.
//
// Absent is not an error: sessions predating the record, and a session
// whose first fixture has not started yet, simply report an unknown
// mode — which a reader must show as unknown rather than assume was
// live.
func ApplySessionMeta(session *Session) {
	meta, err := runner.ReadSessionMeta(session.Dir)
	if err != nil || meta == nil {
		return
	}

	session.Mode = meta.Mode
	session.Planned = meta.Planned
}

// IsFixtureDir reports whether a directory holds a reportable fixture
// run.
//
// A fixture counts if the engine logged anything OR the harness recorded
// an outcome. Requiring the engine log alone silently dropped every
// fixture that legitimately starts no engine — `--help`, and any run
// refused during validation — from a report whose whole job is to list
// what ran.
func IsFixtureDir(dir string) bool {
	return exists(EngineLogPath(dir)) || exists(HarnessRecordPath(dir))
}
