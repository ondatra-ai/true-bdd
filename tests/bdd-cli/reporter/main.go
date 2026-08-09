// Command reporter renders a self-contained HTML run report for a
// true-bdd BDD fixture session.
//
// The report's spine is a gap-free accounting of each fixture's wall
// clock: every slice is tagged DETERMINISTIC (Go code whose duration is
// a property of the machine) or NON-DETERMINISTIC (a model deciding how
// long to take). The two must sum to the wall clock `go test` measured —
// the per-fixture drift is printed so a hole in the model is visible
// rather than hidden in a residual.
//
// Sources:
//
//  1. tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json — the
//     engine's own JSON log. Every AI turn leaves "Dispatching AI turn"
//     (turn/role/cli/model), "AI turn usage" (claude only: cost and
//     tokens) and "AI turn returned"/"AI turn failed" (duration_ms). The
//     spans BETWEEN those records are the engine's deterministic work.
//
//  2. The verbose `go test` output. Two things live only here: the
//     per-fixture wall clock and verdict, and the harness judge's own
//     slog records — the judge runs in the test process, so its cost
//     never reaches the engine log.
//
//  3. The fixture's rendered prompt artifacts, for the runner's
//     <startup> marker and for failure blocks that arrived empty.
//
// Produce the input it wants like this:
//
//	go test -tags bdd ./tests/bdd-cli/... -v -timeout 30m > tmp/bdd-run.log 2>&1
//	go run ./tests/bdd-cli/reporter
//
// Requires the engine telemetry that router.go and claude_provider.go
// emit — per-turn duration_ms / role and the "AI turn usage" record.
// Against a log without those, durations and costs come out empty.
package main

import (
	"errors"
	"flag"
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

// options carries the parsed CLI flags.
type options struct {
	Session string
	GoTest  string
	Out     string
}

// Default paths, all relative to the repo root.
const (
	defaultRunsDir = "tmp/test_run"
	defaultGoTest  = "tmp/bdd-run.log"
	defaultOut     = "tmp/bdd-report.html"
	outputPerm     = 0o644
)

func main() {
	err := run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "reporter: %v\n", err)

		os.Exit(1)
	}
}

// run parses flags, builds the report and writes it.
func run() error {
	opts := parseFlags()

	// Anchoring on the repo root rather than the working directory keeps
	// the output identical whether this is run from the repo root, from
	// this package's directory, or by an editor with a third cwd.
	repoRoot, err := runner.FindRepoRoot()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}

	session, err := resolveSession(repoRoot, opts.Session)
	if err != nil {
		return err
	}

	gotest, err := LoadGoTestLog(atRoot(repoRoot, opts.GoTest))
	if err != nil {
		return err
	}

	fixtures, err := LoadFixtures(session, repoRoot, gotest)
	if err != nil {
		return err
	}

	if len(fixtures) == 0 {
		return fmt.Errorf("%w: %s", errNoFixtures, session)
	}

	out := atRoot(repoRoot, opts.Out)
	sessionName := filepath.Base(session)

	document := NewRenderer(fixtures, sessionName, out).Render()

	err = os.WriteFile(out, []byte(document), outputPerm)
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	written, err := writeDetailPages(fixtures, sessionName, out)
	if err != nil {
		return err
	}

	reportDrift(fixtures)

	_, _ = fmt.Fprintf(os.Stdout, "wrote %s  (%d fixture(s), %d turns, %d detail page(s))\n",
		out, len(fixtures), countTurns(fixtures), written)

	return nil
}

// writeDetailPages writes one page per fixture next to the index, so the
// pair can be moved or served together.
func writeDetailPages(fixtures []*Fixture, session, indexPath string) (int, error) {
	dir := filepath.Dir(indexPath)
	indexHref := filepath.Base(indexPath)

	for _, fixture := range fixtures {
		name := detailFileName(indexPath, fixture.Name)
		page := NewDetailRenderer(fixture, session, indexHref).Render()

		err := os.WriteFile(filepath.Join(dir, name), []byte(page), outputPerm)
		if err != nil {
			return 0, fmt.Errorf("write detail page for %s: %w", fixture.Name, err)
		}
	}

	return len(fixtures), nil
}

// parseFlags reads the command line.
func parseFlags() options {
	var opts options

	flag.StringVar(&opts.Session, "session", "",
		"run session directory to report on (default: newest under "+defaultRunsDir+")")
	flag.StringVar(&opts.GoTest, "gotest", defaultGoTest,
		"verbose `go test` output, for wall clocks, verdicts and judge cost")
	flag.StringVar(&opts.Out, "out", defaultOut, "where to write the HTML report")
	flag.Parse()

	return opts
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

// reportDrift prints how far each fixture's accounted timeline lands
// from the wall clock `go test` measured. This is the report's own
// self-check: a drift that is not near zero means the phase model has a
// hole, and the timeline is lying about where the time went.
func reportDrift(fixtures []*Fixture) {
	for _, fixture := range fixtures {
		if !fixture.HasWall {
			continue
		}

		wall := fixture.Wall.Seconds()

		drift := wall - fixture.PhaseTotal
		if drift < 0 {
			drift = -drift
		}

		_, _ = fmt.Fprintf(os.Stdout, "  %s: wall %.2fs, phases %.2fs, drift %.3fs\n",
			fixture.Name, wall, fixture.PhaseTotal, drift)
	}
}

// countTurns totals the AI turns across every fixture.
func countTurns(fixtures []*Fixture) int {
	total := 0
	for _, fixture := range fixtures {
		total += len(fixture.Turns)
	}

	return total
}
