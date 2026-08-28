package reporter

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// discoveryEndMarkers are the records the engine writes immediately AFTER
// runner.Discover returns. Log-derived rather than the runner artifact's
// mtime: a measurement that changes when a tmpdir is re-run by hand is worse than none.
func discoveryEndMarkers() []string {
	return []string{"Loading full checklist", "Loaded prompts"}
}

// frameworkPlaywright is the engine's name for the playwright runner
// (testrunner.FrameworkPlaywright), as it reaches the report through
// the exit record and through the prompts the run left behind.
const frameworkPlaywright = "playwright"

// startupMarker is the synthetic subject the playwright runner emits
// when the suite could not start at all
// (playwright_runner.go: playwrightStartupMarker).
const startupMarker = "<startup>"

// startupOutcome explains a discovery span that ran no tests. Phrased as
// a claim about cost because that is what it changes: the slice is the
// runner failing to start, not the suite executing.
const startupOutcome = "the engine synthesized its startup marker subject, so the " +
	"runner returned zero real test failures — the suite never " +
	"started, making this startup cost, not test-execution cost"

// emptyFailurePattern finds a prompt that describes a failing test but
// carries no failure text: playwright reports a webServer startup failure
// in its JSON report's top-level errors[], not on stderr, so a synthesized startup subject can reach the model empty.
var emptyFailurePattern = regexp.MustCompile("(?i)Last Failure Output\\s*\n+```[a-z]*\\s*\n\\s*```")

// Discovery is what the engine's test-run span was, and whether any test
// actually ran inside it.
type Discovery struct {
	Framework string
	End       time.Time
	Outcome   string
	Found     bool
}

// Fixture is one fixture's run, assembled from every source.
type Fixture struct {
	Name    string
	Command string
	Dir     string
	// RelDir is Dir relative to the repo root, so every path the report
	// shows can be pasted straight into an editor or shell from the repo
	// root — an absolute path is machine-specific.
	RelDir string

	Turns []*Turn
	First time.Time
	Last  time.Time

	Wall    time.Duration
	HasWall bool
	// Record is the harness's own record of the run, verbatim, when one was
	// written (nil for a fixture still in flight or predating the record).
	// Everything below is derived from it; the server reads the rest straight off this.
	Record *runner.HarnessRecord
	// HasRecord decides what an empty Diff means: the record carries a
	// diff for every fixture (empty means the run wrote nothing), but the
	// legacy `go test` scrape only ever printed one for a failure (empty means unknown).
	HasRecord bool
	Verdict   string
	Judge     *JudgeCall
	Discovery Discovery
	Meta      RunMetadata
	// Checklist is the run's own copy of the checklist it walked,
	// flattened. It supplies the section names and the `F:` presence no
	// artifact filename carries.
	Checklist ChecklistDoc
	Manifest  *Manifest
	// TestRuns is every framework-runner subprocess the run spawned,
	// with its captured output. Empty for a run predating the engine's
	// exit record.
	TestRuns []TestRun
	// Diff and Failures come from the harness's own failure dump, so
	// they exist only for a fixture that failed.
	Diff     []string
	Failures []string

	EmptyFailurePrompts []string

	ModelSeconds float64
	Cost         float64
	Tokens       int

	Phases     []Phase
	PhaseTotal float64
}

// LoadFixtureDir builds one Fixture and its phase timeline from a fixture's
// run directory. Exported so the report server can reload one at a time: a
// fixture is final once its record exists (HarnessRecordPath), so a session can be cached.
func LoadFixtureDir(dir, name, repoRoot string) (*Fixture, error) {
	logPath := EngineLogPath(dir)

	log := &EngineLog{}

	if exists(logPath) {
		// A log that cannot be read degrades this fixture to "no turns
		// recorded" rather than failing the scan (same stance as applyHarnessRecord).
		loaded, err := loadEngineLog(logPath)
		if err == nil {
			log = loaded
		}
	}

	manifest := loadManifest(repoRoot, name, dir)

	fixture := &Fixture{
		Name:                name,
		Command:             manifest.Command,
		Dir:                 dir,
		RelDir:              relativeToRoot(repoRoot, dir),
		Manifest:            manifest,
		Turns:               log.Turns(dir),
		Meta:                log.Metadata(),
		TestRuns:            log.TestRuns(dir),
		First:               log.First,
		Last:                log.Last,
		EmptyFailurePrompts: findEmptyFailurePrompts(dir),
	}

	applyHarnessRecord(fixture, dir)
	nameOperations(fixture, dir)

	for _, turn := range fixture.Turns {
		fixture.ModelSeconds += turn.Seconds()
		fixture.Cost += turn.CostUSD
		fixture.Tokens += turn.Tokens.Total()
	}

	if len(fixture.Turns) > 0 && !fixture.First.IsZero() {
		fixture.Discovery = findDiscovery(dir, log, fixture.First, fixture.Turns[0].Started)
	}

	fixture.Phases = buildPhases(fixture)
	for _, phase := range fixture.Phases {
		fixture.PhaseTotal += phase.Seconds
	}

	return fixture, nil
}

// nameOperations gives every turn its operation name, reading the run's OWN
// copy of the checklist rather than the working tree's — a fixture may have
// filtered or overridden it, and only the run's copy has the indices used.
func nameOperations(fixture *Fixture, dir string) {
	fixture.Checklist = loadChecklistDoc(dir, fixture.Meta.ChecklistPath)

	for _, turn := range fixture.Turns {
		turn.Op = describeTurn(turn, fixture.Checklist, fixture.Meta.ChecklistCommand)
	}
}

// relativeToRoot expresses a path relative to the repo root, falling
// back to the absolute path when the two share no ancestor.
func relativeToRoot(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}

	return rel
}

// findDiscovery bounds the engine's test-run span and says whether tests
// ran. The duration comes from the log; the verdict comes from the
// prompt artifacts the engine wrote during the run.
func findDiscovery(dir string, log *EngineLog, windowStart, windowEnd time.Time) Discovery {
	end := firstMarkerAfter(log, windowEnd)
	if end.IsZero() || !end.After(windowStart) {
		return Discovery{}
	}

	discovery := Discovery{Framework: "test runner", End: end, Found: true}

	for _, path := range promptArtifacts(dir) {
		content, err := disk.Read(path)
		if err != nil {
			continue
		}

		text := string(content)
		if !strings.Contains(text, startupMarker) {
			continue
		}

		discovery.Framework = "runner"
		if strings.Contains(strings.ToLower(text), frameworkPlaywright) {
			discovery.Framework = frameworkPlaywright
		}

		discovery.Outcome = startupOutcome

		break
	}

	return discovery
}

// firstMarkerAfter returns the first discovery-end marker at or before
// the window's end.
func firstMarkerAfter(log *EngineLog, windowEnd time.Time) time.Time {
	markers := discoveryEndMarkers()

	for index := range log.Records {
		record := &log.Records[index]
		if record.At.IsZero() || record.At.After(windowEnd) {
			continue
		}

		for _, marker := range markers {
			if record.Msg == marker {
				return record.At
			}
		}
	}

	return time.Time{}
}

// findEmptyFailurePrompts lists prompts whose failure block arrived
// empty — invisible in the timings, but decisive for what the fix turn
// had to work with.
func findEmptyFailurePrompts(dir string) []string {
	var hits []string

	for _, path := range promptArtifacts(dir) {
		content, err := disk.Read(path)
		if err != nil {
			continue
		}

		if emptyFailurePattern.Match(content) {
			hits = append(hits, filepath.Base(path))
		}
	}

	return hits
}

// promptArtifacts lists the run's rendered user prompts, sorted.
func promptArtifacts(dir string) []string {
	matches, err := filepath.Glob(filepath.Join(dir, "tmp", "*", "*user.txt"))
	if err != nil {
		return nil
	}

	sort.Strings(matches)

	return matches
}

// exists reports whether a path is present, treating any stat error as
// absent — the caller only ever asks so it can skip or substitute.
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	// Only "not there" counts as absent — a permission/I/O error means the
	// file may exist and we can't see it, and reporting that as absent would
	// silently drop the fixture from a report whose whole job is to list what ran.
	return !errors.Is(err, fs.ErrNotExist)
}
