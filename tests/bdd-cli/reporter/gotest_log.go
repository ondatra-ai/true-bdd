package reporter

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// gotestVerdictPattern matches `go test -v`'s per-subtest verdict line.
// The wall clock it reports is the only measurement of a fixture's total
// that exists anywhere: the engine cannot see the prep, snapshot, diff
// or judge that bracket its own process.
var gotestVerdictPattern = regexp.MustCompile(
	`^\s*--- (PASS|FAIL|SKIP): TestBDDFixtures/(\S+) \(([\d.]+)s\)`,
)

// gotestUsagePattern matches the harness judge's own usage record. The
// judge runs inside the test process, where slog is unconfigured and
// falls back to its default text format on stderr — so this cost never
// reaches the engine log and would otherwise go unbilled.
var gotestUsagePattern = regexp.MustCompile(
	`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) INFO AI turn usage (.+)$`,
)

// gotestTimeLayout is the layout of slog's default text-handler stamp.
const gotestTimeLayout = "2006/01/02 15:04:05"

// FixtureVerdict is what `go test` reported for one fixture.
type FixtureVerdict struct {
	Verdict string
	Wall    time.Duration
	// Diff and Failures are printed only when a fixture fails — the
	// harness dumps them from dumpRun. A passing fixture legitimately
	// has neither, which is why their absence is stated rather than
	// rendered as an empty list.
	Diff     []string
	Failures []string
}

// Patterns for the per-fixture detail the harness prints on failure.
var (
	gotestRunPattern = regexp.MustCompile(
		`^\s*=== RUN\s+TestBDDFixtures/(\S+)`,
	)
	gotestDiffPattern = regexp.MustCompile(
		`^\s*bdd_test\.go:\d+:\s+((?:created|modified|deleted) .+)$`,
	)
	gotestFailurePattern = regexp.MustCompile(
		`^\s*bdd_test\.go:\d+:\s+- (.+)$`,
	)
)

// GoTestLog is the parsed `go test -v` output — the legacy fallback for
// sessions recorded before the harness wrote its own record.
type GoTestLog struct {
	Verdicts map[string]FixtureVerdict
	Judges   []JudgeCall
	// current is the fixture whose subtest output is being read, so the
	// failure detail printed between RUN and the verdict line lands on
	// the right fixture in a multi-fixture run.
	current string
	// judgeCursor is how many judge calls have already been claimed.
	// These records carry no fixture name, so the only pairing available
	// here is order — consumed as fixtures fall back to this source,
	// rather than indexed by position in the session directory.
	judgeCursor int
}

// applyGoTestVerdict fills a fixture from the legacy log.
func applyGoTestVerdict(fixture *Fixture, log *GoTestLog) {
	verdict, ok := log.Verdicts[fixture.Name]
	if !ok {
		return
	}

	fixture.Wall = verdict.Wall
	fixture.HasWall = true
	fixture.Verdict = verdict.Verdict
	fixture.Diff = verdict.Diff
	fixture.Failures = verdict.Failures

	if log.judgeCursor < len(log.Judges) {
		fixture.Judge = &log.Judges[log.judgeCursor]
		log.judgeCursor++
	}
}

// emptyGoTestLog is the fallback source when no legacy log was asked
// for: every lookup misses, so every fixture falls through to its
// harness record.
func emptyGoTestLog() *GoTestLog {
	return &GoTestLog{Verdicts: map[string]FixtureVerdict{}}
}

// loadGoTestLog parses the verbose test output. A missing file is not an
// error: the report still renders from the engine logs alone, just
// without wall clocks, verdicts or judge costs.
func loadGoTestLog(path string) (*GoTestLog, error) {
	log := emptyGoTestLog()

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return log, nil
		}

		return nil, fmt.Errorf("open go test log: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, initialScanBuffer), maxScanTokenSize)

	for scanner.Scan() {
		log.consume(scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("read go test log: %w", err)
	}

	return log, nil
}

// consume folds one output line into the parsed log.
func (l *GoTestLog) consume(line string) {
	if match := gotestRunPattern.FindStringSubmatch(line); match != nil {
		l.current = match[1]

		return
	}

	if match := gotestVerdictPattern.FindStringSubmatch(line); match != nil {
		l.consumeVerdict(match)

		return
	}

	if match := gotestDiffPattern.FindStringSubmatch(line); match != nil {
		l.appendDetail(func(verdict *FixtureVerdict) {
			verdict.Diff = append(verdict.Diff, match[1])
		})

		return
	}

	if match := gotestFailurePattern.FindStringSubmatch(line); match != nil {
		l.appendDetail(func(verdict *FixtureVerdict) {
			verdict.Failures = append(verdict.Failures, match[1])
		})

		return
	}

	if match := gotestUsagePattern.FindStringSubmatch(line); match != nil {
		l.consumeJudge(match[usageStampGroup], match[usageFieldsGroup])
	}
}

// consumeVerdict records a subtest's outcome, keeping any detail already
// gathered for it — the harness prints the diff BEFORE the verdict line.
func (l *GoTestLog) consumeVerdict(match []string) {
	seconds, err := strconv.ParseFloat(match[verdictSecondsGroup], 64)
	if err != nil {
		return
	}

	name := match[verdictNameGroup]

	verdict := l.Verdicts[name]
	verdict.Verdict = match[verdictKindGroup]
	verdict.Wall = time.Duration(seconds * float64(time.Second))

	l.Verdicts[name] = verdict
}

// appendDetail applies a mutation to the fixture currently being read.
func (l *GoTestLog) appendDetail(apply func(*FixtureVerdict)) {
	if l.current == "" {
		return
	}

	verdict := l.Verdicts[l.current]
	apply(&verdict)
	l.Verdicts[l.current] = verdict
}

// Submatch indices for the two patterns above.
const (
	verdictKindGroup    = 1
	verdictNameGroup    = 2
	verdictSecondsGroup = 3
	usageStampGroup     = 1
	usageFieldsGroup    = 2
)

// consumeJudge records one judge call from its `key=value` field list.
func (l *GoTestLog) consumeJudge(stamp, fields string) {
	// slog's default text handler writes local time with no offset, so
	// the reader's zone is the only correct assumption — and the only
	// one that puts this stamp on the same axis as the engine log's
	// offset-carrying records.
	//nolint:gosmopolitan // the stamp is local-by-construction; see above
	at, err := time.ParseInLocation(gotestTimeLayout, stamp, time.Local)
	if err != nil {
		return
	}

	call := JudgeCall{At: at}
	parsed := parseSlogFields(fields)

	if cost, ok := parsed["cost_usd"]; ok {
		call.CostUSD, _ = strconv.ParseFloat(cost, 64)
	}

	for _, key := range tokenKeys() {
		value, ok := parsed[key]
		if !ok {
			continue
		}

		count, convErr := strconv.Atoi(value)
		if convErr != nil {
			continue
		}

		call.Tokens += count
	}

	l.Judges = append(l.Judges, call)
}

// parseSlogFields splits slog's text-handler `key=value key=value` tail.
func parseSlogFields(fields string) map[string]string {
	out := map[string]string{}

	for _, pair := range strings.Fields(fields) {
		key, value, ok := strings.Cut(pair, "=")
		if ok {
			out[key] = value
		}
	}

	return out
}
