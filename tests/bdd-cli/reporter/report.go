// Package reporter renders a self-contained HTML run report for a
// true-bdd BDD fixture session.
//
// The report's spine is a gap-free accounting of each fixture's wall
// clock: every slice is tagged DETERMINISTIC (Go code whose duration is
// a property of the machine) or NON-DETERMINISTIC (a model deciding how
// long to take). The two must sum to the wall clock the harness
// measured — the per-fixture drift is reported so a hole in the model is
// visible rather than hidden in a residual.
//
// Sources:
//
//  1. tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json — the
//     engine's own JSON log. Every AI turn leaves "Dispatching AI turn"
//     (turn/role/cli/model), "AI turn usage" (claude only: cost and
//     tokens) and "AI turn returned"/"AI turn failed" (duration_ms). The
//     spans BETWEEN those records are the engine's deterministic work.
//
//  2. tmp/test_run/<session>/<fixture>/bdd-cli-logs/harness.json — the
//     harness's own record of the run. Four things live only here,
//     because the engine cannot see them from inside its own process:
//     the fixture's wall clock, its verdict, the file diff, and what the
//     harness judge spent.
//
//  3. The fixture's rendered prompt artifacts, for the runner's
//     <startup> marker and for failure blocks that arrived empty.
//
//  4. The fixture manifest, via runner.LoadFixture.
//
// The BDD suite calls Render after every fixture, so a report exists
// while a long run is still going. Rendering an older session by hand:
//
//	go run ./tests/bdd-cli/cmd/reporter
//
// Requires the engine telemetry that router.go and claude_provider.go
// emit — per-turn duration_ms / role and the "AI turn usage" record.
// Against a log without those, durations and costs come out empty.
package reporter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// errNoSessions is returned when there is nothing to report on.
var errNoSessions = errors.New("no fixture run sessions found — run the BDD suite first")

// errNoFixtures is returned when a session holds no engine logs.
var errNoFixtures = errors.New("session contains no fixture engine logs")

// Default paths, all relative to the repo root.
const (
	defaultRunsDir = "tmp/test_run"
	defaultOutDir  = "tmp/test_report"
)

// Options selects what one render reads and where it writes. The zero
// value is the standalone default: the newest session, no legacy log,
// tmp/test_report — every path resolved against the repo root, so the
// working directory never changes the answer.
type Options struct {
	// Session is the run-session directory. Empty picks the newest
	// directory under tmp/test_run. The BDD harness passes the session
	// root it already created rather than relying on that guess: a
	// second suite started inside the same second would otherwise send
	// the report at the wrong session.
	Session string
	// GoTest is a verbose `go test` log to fall back on for fixtures
	// that have no harness record — sessions captured before the
	// harness wrote one. Empty means do not read one at all. Opt-in on
	// purpose: the harness now configures slog, which changes the
	// format this log's judge records are scraped from, so a legacy
	// log left active by default would quietly drop the judge's cost
	// while still producing a complete-looking timeline.
	GoTest string
	// OutDir receives index.html plus one <fixture>.html beside it.
	OutDir string
}

// FixtureDrift is one fixture's accounting self-check: how far its phase
// timeline lands from the wall clock the harness measured. A number that
// is not near zero means the phase model has a hole, and the timeline is
// lying about where the time went.
type FixtureDrift struct {
	Name   string
	Wall   float64
	Phases float64
	Drift  float64
}

// Summary is what one render produced, so each caller can report it in
// its own voice. Deliberately scalar: it carries no *Fixture, so
// printing a summary never hands out the whole object graph.
type Summary struct {
	Session   string
	IndexPath string
	Fixtures  int
	Turns     int
	Pages     int
	Drift     []FixtureDrift
}

// Render reads one session and writes the flat page set to OutDir,
// overwriting whatever a previous run left there. Safe to call after
// every fixture: it rebuilds the whole set from the session directory
// each time, so a partial session yields a partial report rather than a
// stale one.
//
// Deliberately prints nothing. The standalone command writes to stdout;
// the BDD harness writes through t.Logf. A library that printed would
// have to pick one.
func Render(opts Options) (Summary, error) {
	// Anchoring on the repo root rather than the working directory keeps
	// the output identical whether this runs from the repo root, from a
	// test package's directory, or from an editor with a third cwd.
	repoRoot, err := runner.FindRepoRoot()
	if err != nil {
		return Summary{}, fmt.Errorf("find repo root: %w", err)
	}

	session, err := resolveSession(repoRoot, opts.Session)
	if err != nil {
		return Summary{}, err
	}

	gotest, err := resolveGoTestLog(repoRoot, opts.GoTest)
	if err != nil {
		return Summary{}, err
	}

	fixtures, err := loadFixtures(session, repoRoot, gotest)
	if err != nil {
		return Summary{}, err
	}

	if len(fixtures) == 0 {
		return Summary{}, fmt.Errorf("%w: %s", errNoFixtures, session)
	}

	name := filepath.Base(session)
	outDir := atRoot(repoRoot, orDefault(opts.OutDir, defaultOutDir))

	pages, err := writePages(fixtures, name, outDir)
	if err != nil {
		return Summary{}, err
	}

	return Summary{
		Session:   name,
		IndexPath: filepath.Join(outDir, indexFileName),
		Fixtures:  len(fixtures),
		Turns:     countTurns(fixtures),
		Pages:     pages,
		Drift:     driftOf(fixtures),
	}, nil
}

// resolveGoTestLog loads the legacy fallback log, or nothing when none
// was asked for.
func resolveGoTestLog(repoRoot, path string) (*GoTestLog, error) {
	if path == "" {
		return emptyGoTestLog(), nil
	}

	return loadGoTestLog(atRoot(repoRoot, path))
}

// orDefault falls back when a caller left an option blank.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// atRoot resolves a path argument against the repo root, not the cwd.
func atRoot(repoRoot, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(repoRoot, path)
}

// resolveSession picks the session to report on: the one named, or the
// newest by directory name (which is a sortable timestamp).
func resolveSession(repoRoot, named string) (string, error) {
	if named != "" {
		return atRoot(repoRoot, named), nil
	}

	runsDir := filepath.Join(repoRoot, defaultRunsDir)

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errNoSessions, runsDir)
	}

	var names []string

	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	if len(names) == 0 {
		return "", fmt.Errorf("%w: %s", errNoSessions, runsDir)
	}

	sort.Strings(names)

	return filepath.Join(runsDir, names[len(names)-1]), nil
}

// driftOf measures how far each fixture's accounted timeline lands from
// the wall clock the harness recorded. This is the report's own
// self-check, handed to the caller rather than printed.
func driftOf(fixtures []*Fixture) []FixtureDrift {
	var drifts []FixtureDrift

	for _, fixture := range fixtures {
		if !fixture.HasWall {
			continue
		}

		wall := fixture.Wall.Seconds()

		drift := wall - fixture.PhaseTotal
		if drift < 0 {
			drift = -drift
		}

		drifts = append(drifts, FixtureDrift{
			Name:   fixture.Name,
			Wall:   wall,
			Phases: fixture.PhaseTotal,
			Drift:  drift,
		})
	}

	return drifts
}

// countTurns totals the AI turns across every fixture.
func countTurns(fixtures []*Fixture) int {
	total := 0
	for _, fixture := range fixtures {
		total += len(fixture.Turns)
	}

	return total
}
