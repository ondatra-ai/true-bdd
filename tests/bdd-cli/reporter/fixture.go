package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// discoveryEndMarkers are the records the engine writes immediately
// AFTER runner.Discover returns. Discovery itself logs nothing, so one
// of these closes its span.
//
// Deliberately log-derived rather than taken from the runner artifact's
// mtime: a preserved tmpdir can be re-run by hand later, and a
// measurement that changes when someone inspects it is worse than none.
func discoveryEndMarkers() []string {
	return []string{"Loading full checklist", "Loaded prompts"}
}

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
// carries no failure text. Playwright reports a webServer startup
// failure in its JSON report's top-level errors[], not on stderr, so a
// synthesized startup subject can reach the model with an empty block.
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

	Turns []*Turn
	First time.Time
	Last  time.Time

	Wall      time.Duration
	HasWall   bool
	Verdict   string
	Judge     *JudgeCall
	Discovery Discovery
	Meta      RunMetadata
	Manifest  *Manifest
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

// LoadFixtures assembles every fixture in a session directory that left
// an engine log behind.
func LoadFixtures(sessionDir, repoRoot string, gotest *GoTestLog) ([]*Fixture, error) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var fixtures []*Fixture

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(sessionDir, entry.Name())

		logPath := filepath.Join(dir, "tmp", "true-bdd.log.json")

		_, statErr := os.Stat(logPath)
		if statErr != nil {
			continue
		}

		fixture, loadErr := loadFixture(dir, entry.Name(), repoRoot, logPath, gotest, len(fixtures))
		if loadErr != nil {
			return nil, loadErr
		}

		fixtures = append(fixtures, fixture)
	}

	return fixtures, nil
}

// loadFixture builds one Fixture and its phase timeline.
func loadFixture(
	dir, name, repoRoot, logPath string,
	gotest *GoTestLog,
	index int,
) (*Fixture, error) {
	log, err := LoadEngineLog(logPath)
	if err != nil {
		return nil, err
	}

	manifest := LoadManifest(repoRoot, name)

	fixture := &Fixture{
		Name:                name,
		Command:             manifest.Command,
		Dir:                 dir,
		Manifest:            manifest,
		Turns:               log.Turns(dir),
		Meta:                log.Metadata(),
		First:               log.First,
		Last:                log.Last,
		EmptyFailurePrompts: findEmptyFailurePrompts(dir),
	}

	if verdict, ok := gotest.Verdicts[name]; ok {
		fixture.Wall = verdict.Wall
		fixture.HasWall = true
		fixture.Verdict = verdict.Verdict
		fixture.Diff = verdict.Diff
		fixture.Failures = verdict.Failures
	}

	// Judge calls carry no fixture name, so they are paired in order.
	// Fixtures run sequentially, which makes that exact.
	if index < len(gotest.Judges) {
		fixture.Judge = &gotest.Judges[index]
	}

	for _, turn := range fixture.Turns {
		fixture.ModelSeconds += turn.Seconds()
		fixture.Cost += turn.CostUSD
		fixture.Tokens += turn.Tokens.Total()
	}

	if len(fixture.Turns) > 0 && !fixture.First.IsZero() {
		fixture.Discovery = findDiscovery(dir, log, fixture.First, fixture.Turns[0].Started)
	}

	fixture.Phases = BuildPhases(fixture)
	for _, phase := range fixture.Phases {
		fixture.PhaseTotal += phase.Seconds
	}

	return fixture, nil
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
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		text := string(content)
		if !strings.Contains(text, startupMarker) {
			continue
		}

		discovery.Framework = "runner"
		if strings.Contains(strings.ToLower(text), "playwright") {
			discovery.Framework = "playwright"
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
		content, err := os.ReadFile(path)
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
