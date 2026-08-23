package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// playwrightTitleSeparator joins nested describe titles and the leaf test
// title into the chain on FailingTest.TestName — matches the " > "
// Playwright's own CLI uses for `--list` output.
const playwrightTitleSeparator = " > "

// playwrightNameSeparator joins the spec file path with the test title
// chain inside FailingTest.TestName so the (file, title) pair survives
// round-tripping through RunOne.
const playwrightNameSeparator = "::"

// playwrightStartupMarker is the FailingTest.TestName for the synthetic
// entry Discover emits on a webServer/fixture setup failure. RunOne
// reruns the whole suite for it rather than grep a non-existent title.
const playwrightStartupMarker = "<startup>"

// ErrInvalidPlaywrightName signals that a FailingTest.TestName intended
// for the Playwright runner is not in the expected
// "<file>::<title chain>" shape.
var ErrInvalidPlaywrightName = errors.New("invalid playwright TestName shape")

// playwrightRegexMeta enumerates the regex meta-characters that must be
// escaped when building a `--grep` filter from a free-form test title.
var playwrightRegexMeta = regexp.MustCompile(`[\\^$.|?*+()[\]{}]`)

// PlaywrightRunner runs `npx playwright test --reporter=json` and
// decodes the trailing JSON document into FailingTest values.
type PlaywrightRunner struct {
	artifacts *Artifacts
}

// NewPlaywrightRunner builds a PlaywrightRunner writing its captured
// output through artifacts, which may be nil to capture nothing.
func NewPlaywrightRunner(artifacts *Artifacts) *PlaywrightRunner {
	return &PlaywrightRunner{artifacts: artifacts}
}

// Discover runs the full Playwright suite and returns one FailingTest per
// failed test. When Playwright exits non-zero with none landing in the
// JSON report, it synthesizes a startup-marker FailingTest instead of silently converging on zero.
func (r *PlaywrightRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, suite string,
) ([]*FailingTest, error) {
	argv, err := SplitCommand(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("playwright command for %s/%s: %w", service, suite, err)
	}

	stdout, stderr, runErr := r.exec(ctx, CommandDir(cfg), PhaseDiscover, argv)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("playwright discovery under %s failed: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	report, err := parsePlaywrightReport(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("playwright report parse failed: %w", err)
	}

	failures := playwrightReportToFailingTests(report, service, suite)

	logPlaywrightReport(PhaseDiscover, report, len(failures))

	if runErr != nil && len(failures) == 0 {
		failures = append(failures, newPlaywrightStartupFailure(service, suite, cfg, stderr.String()))
	}

	for _, failure := range failures {
		failure.RunnerConfig = cfg
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].ID < failures[j].ID })

	return failures, nil
}

// newPlaywrightStartupFailure builds the synthetic FailingTest for the
// startup-failure case. The TestName is the well-known marker so RunOne
// can dispatch to the whole-suite rerun branch.
func newPlaywrightStartupFailure(
	service, suite string,
	cfg Config,
	stderrText string,
) *FailingTest {
	return &FailingTest{
		ID:            BuildID(service, suite, FrameworkPlaywright, playwrightStartupMarker),
		Service:       service,
		Suite:         suite,
		Framework:     FrameworkPlaywright,
		TestName:      playwrightStartupMarker,
		FilePath:      cfg.Path,
		FailureOutput: TruncateTail(stderrText, FailureOutputCap),
	}
}

// RunOne re-executes one failing test via `--grep` (ANDed with the paths)
// appended to the suite's command. Never appends the spec file: Playwright
// ORs positional path filters, so an added file would widen the rerun instead of isolating it.
func (r *PlaywrightRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	if failingTest.TestName == playwrightStartupMarker {
		return r.runOneStartup(ctx, failingTest)
	}

	_, title, err := splitPlaywrightName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	argv, err := commandArgv(failingTest)
	if err != nil {
		return false, "", err
	}

	grep := playwrightGrep(title)
	argv = append(argv, "--grep", grep)

	stdout, stderr, runErr := r.exec(ctx, CommandDir(failingTest.RunnerConfig), PhaseRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("playwright rerun of %s failed: %w",
			failingTest.TestName, runErr)
	}

	report, parseErr := parsePlaywrightReport(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("playwright rerun report parse failed", "error", parseErr)

		return false, stdout.String(), nil
	}

	rerunFailures := playwrightReportToFailingTests(report, "", "")

	logPlaywrightReport(PhaseRerun, report, len(rerunFailures))

	if playwrightRanNothing(report) {
		return false, stdout.String(), fmt.Errorf("%w: playwright --grep %q matched no test for %s",
			ErrRerunSelectedNoTests, grep, failingTest.TestName)
	}

	for _, failure := range rerunFailures {
		if failure.TestName == failingTest.TestName {
			return false, failure.FailureOutput, nil
		}
	}

	return true, "", nil
}

// playwrightGrep builds the `--grep` pattern for one test's title chain.
// Per Playwright's source (`Suite._grepTitleWithTags`), it's tested
// unanchored against the SPACE-joined title path — `^chain$` matches nothing and every rerun reports as a pass.
func playwrightGrep(titleChain string) string {
	spaced := strings.ReplaceAll(titleChain, playwrightTitleSeparator, " ")

	return playwrightRegexMeta.ReplaceAllString(spaced, `\$0`)
}

// playwrightRanNothing reports whether the rerun executed no test at all.
// Playwright emits its stats block even then, so an all-zero block
// separates "filter selected nothing" from "test passed" — without it, the former reads as a fix that worked.
func playwrightRanNothing(report *dto.PlaywrightReport) bool {
	stats := report.Stats

	return stats.Expected+stats.Unexpected+stats.Flaky+stats.Skipped == 0
}

// runOneStartup is the synthetic-marker branch of RunOne. Re-runs the
// whole Playwright suite (no --grep, no file filter) and reports passed
// iff the new run had no per-test failures AND the exec exited zero.
func (r *PlaywrightRunner) runOneStartup(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	argv, err := commandArgv(failingTest)
	if err != nil {
		return false, "", err
	}

	stdout, stderr, runErr := r.exec(ctx, CommandDir(failingTest.RunnerConfig), PhaseStartupRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("playwright startup rerun failed: %w", runErr)
	}

	report, parseErr := parsePlaywrightReport(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("playwright startup rerun report parse failed", "error", parseErr)

		return false, stdout.String(), nil
	}

	rerunFailures := playwrightReportToFailingTests(report, "", "")

	logPlaywrightReport(PhaseStartupRerun, report, len(rerunFailures))

	if runErr == nil && len(rerunFailures) == 0 {
		return true, "", nil
	}

	output := stderr.String()
	if len(rerunFailures) > 0 {
		output = rerunFailures[0].FailureOutput
	}

	return false, TruncateTail(output, FailureOutputCap), nil
}

// exec runs the suite's command with the supplied argv. cwd is the config
// directory so `npx` resolves the local Playwright install (see CommandDir).
// phase labels the invocation in log/filename; non-zero exit is expected on test failure.
func (r *PlaywrightRunner) exec(
	ctx context.Context,
	cwd, phase string,
	argv []string,
) (bytes.Buffer, bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd

	return runLogged(cmd, spawnMeta{
		binary:    argv[0],
		args:      argv[1:],
		framework: FrameworkPlaywright,
		phase:     phase,
		artifacts: r.artifacts,
	})
}

// splitPlaywrightName parses "<file>::<title chain>" into its parts,
// returning ErrInvalidPlaywrightName for malformed inputs.
func splitPlaywrightName(name string) (string, string, error) {
	file, title, ok := strings.Cut(name, playwrightNameSeparator)
	if !ok || file == "" || title == "" {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPlaywrightName, name)
	}

	return file, title, nil
}

// parsePlaywrightReport decodes the JSON document from `--reporter=json`,
// trimming leading log noise Playwright writes before it (unlike Jest,
// which writes only JSON). Lives in `dto` since its camelCase mirrors a third-party wire format.
func parsePlaywrightReport(payload []byte) (*dto.PlaywrightReport, error) {
	start := bytes.IndexByte(payload, '{')
	if start < 0 {
		return &dto.PlaywrightReport{}, nil
	}

	var report dto.PlaywrightReport

	err := json.Unmarshal(payload[start:], &report)
	if err != nil {
		return nil, fmt.Errorf("decode playwright json: %w", err)
	}

	return &report, nil
}

// logPlaywrightReport records what Playwright said about the run: exit
// code alone can't tell "ran and failed" from "died before the first
// test" — stats/errors[] (see Capture) are the only trace once FailingTest drops them.
func logPlaywrightReport(phase string, report *dto.PlaywrightReport, failures int) {
	slog.Debug("Playwright report decoded",
		"phase", phase,
		"start_time", report.Stats.StartTime,
		"duration_ms", int64(report.Stats.Duration),
		"expected", report.Stats.Expected,
		"unexpected", report.Stats.Unexpected,
		"skipped", report.Stats.Skipped,
		"flaky", report.Stats.Flaky,
		"suites", len(report.Suites),
		"run_errors", playwrightErrorMessages(report.Errors),
		"failures_collected", failures,
	)
}

// playwrightErrorMessages flattens the run-level error block to its
// messages. Stacks are omitted: for a webServer failure the stack
// duplicates the message, and this log record indexes the failure, not replaces reading it.
func playwrightErrorMessages(errs []dto.PlaywrightError) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Message)
	}

	return out
}

// playwrightReportToFailingTests walks the suite tree and collects one
// FailingTest per failed result, tagged with service/suite. Free function
// so dto stays passive, matching jestReportToFailingTests.
func playwrightReportToFailingTests(
	report *dto.PlaywrightReport,
	service, suite string,
) []*FailingTest {
	collector := &playwrightCollector{service: service, suite: suite}
	for _, suite := range report.Suites {
		collector.walk(suite, "", suite.File)
	}

	return collector.out
}

// playwrightCollector accumulates FailingTest entries during the suite
// walk. Stateful so the recursive walker can append without threading
// the slice through every call.
type playwrightCollector struct {
	service string
	suite   string
	out     []*FailingTest
}

// walk descends one suite, appending failures from its specs and
// recursing into nested suites. titleChain is the " > "-joined chain
// of describe titles accumulated from the root.
func (c *playwrightCollector) walk(suite dto.PlaywrightSuite, titleChain, file string) {
	nextChain := joinTitle(titleChain, suite.Title)
	nextFile := pickFile(file, suite.File)

	for _, spec := range suite.Specs {
		c.collectSpec(spec, nextChain, pickFile(nextFile, spec.File))
	}

	for _, child := range suite.Suites {
		c.walk(child, nextChain, nextFile)
	}
}

// collectSpec turns one spec into 0..N FailingTest entries (one per
// failed attempt that wasn't already counted).
func (c *playwrightCollector) collectSpec(spec dto.PlaywrightSpec, titleChain, file string) {
	fullTitle := joinTitle(titleChain, spec.Title)

	for _, test := range spec.Tests {
		if !specRunFailed(test.Results) {
			continue
		}

		name := file + playwrightNameSeparator + fullTitle
		c.out = append(c.out, &FailingTest{
			ID:            BuildID(c.service, c.suite, FrameworkPlaywright, name),
			Service:       c.service,
			Suite:         c.suite,
			Framework:     FrameworkPlaywright,
			TestName:      name,
			FilePath:      file,
			FailureOutput: TruncateTail(formatPlaywrightFailure(test.Results), FailureOutputCap),
		})
	}
}

// specRunFailed reports whether at least one attempt for this project
// run ended in failure (and no later attempt recovered to pass).
func specRunFailed(results []dto.PlaywrightResult) bool {
	if len(results) == 0 {
		return false
	}

	final := results[len(results)-1]

	return final.Status == statusFailed || final.Status == statusTimedOut
}

// formatPlaywrightFailure stitches the failed results' errors and
// stdout/stderr into one human-readable block for the prompt.
func formatPlaywrightFailure(results []dto.PlaywrightResult) string {
	var buf strings.Builder

	for idx, result := range results {
		if result.Status != statusFailed && result.Status != statusTimedOut {
			continue
		}

		fmt.Fprintf(&buf, "--- attempt %d: %s ---\n", idx+1, result.Status)

		for _, err := range result.Errors {
			buf.WriteString(err.Message)
			buf.WriteString("\n")

			if err.Stack != "" {
				buf.WriteString(err.Stack)
				buf.WriteString("\n")
			}
		}

		for _, out := range result.Stdout {
			buf.WriteString(out.Text)
		}

		for _, out := range result.Stderr {
			buf.WriteString(out.Text)
		}
	}

	return buf.String()
}

// joinTitle composes two title segments with the " > " separator,
// skipping empty segments so chains don't grow stray leading separators.
func joinTitle(left, right string) string {
	if left == "" {
		return right
	}

	if right == "" {
		return left
	}

	return left + playwrightTitleSeparator + right
}

// pickFile returns the non-empty file path. Playwright sometimes
// repeats `file:` on nested suites; this picks whichever level is set.
func pickFile(parent, child string) string {
	if child != "" {
		return child
	}

	return parent
}
