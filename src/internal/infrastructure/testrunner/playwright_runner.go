package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"bdd-cli/src/internal/infrastructure/testrunner/dto"
)

// playwrightTitleSeparator joins nested describe titles and the leaf
// test title into the chain stored on FailingTest.TestName. Playwright's
// own CLI uses " > " for the same concept in `--list` output, so we
// follow suit.
const playwrightTitleSeparator = " > "

// playwrightNameSeparator joins the spec file path with the test title
// chain inside FailingTest.TestName so the (file, title) pair survives
// round-tripping through RunOne.
const playwrightNameSeparator = "::"

// playwrightStartupMarker is the FailingTest.TestName used for the
// synthetic entry Discover emits when Playwright exits non-zero before
// any per-test failure was captured — typically a webServer / fixture
// setup failure. RunOne re-runs the whole suite for this marker rather
// than trying to grep a non-existent test title.
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
type PlaywrightRunner struct{}

// NewPlaywrightRunner builds a PlaywrightRunner.
func NewPlaywrightRunner() *PlaywrightRunner {
	return &PlaywrightRunner{}
}

// Discover runs the full Playwright suite declared by cfg and returns
// one FailingTest per failed test. When Playwright exits non-zero
// without any per-test failure landing in the JSON report (typically
// because the webServer command itself failed — e.g. `docker compose
// up` couldn't build), Discover synthesizes one startup-marker
// FailingTest so the engine has something to drive Claude against
// instead of silently converging on zero items.
func (r *PlaywrightRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, layer string,
) ([]*FailingTest, error) {
	cwd, configArg, pathArg := playwrightPaths(cfg)

	stdout, stderr, runErr := r.exec(ctx, cwd, "test", "--reporter=json",
		"--config", configArg, pathArg)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("playwright discovery under %s failed: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	report, err := parsePlaywrightReport(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("playwright report parse failed: %w", err)
	}

	failures := playwrightReportToFailingTests(report, service, layer)

	if runErr != nil && len(failures) == 0 {
		failures = append(failures, newPlaywrightStartupFailure(service, layer, cfg, stderr.String()))
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
	service, layer string,
	cfg Config,
	stderrText string,
) *FailingTest {
	return &FailingTest{
		ID:            BuildID(service, layer, FrameworkPlaywright, playwrightStartupMarker),
		Service:       service,
		Layer:         layer,
		Framework:     FrameworkPlaywright,
		TestName:      playwrightStartupMarker,
		FilePath:      cfg.Path,
		FailureOutput: TruncateTail(stderrText, FailureOutputCap),
	}
}

// RunOne re-executes a single failing test in isolation via `--grep`
// regex (anchored to the escaped title) plus the spec file positional
// argument. For the synthetic startup-marker FailingTest, RunOne
// instead re-runs the whole suite — the test title is not a real
// Playwright test name — and reports passed iff the rerun emits zero
// per-test failures with a clean exit.
func (r *PlaywrightRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	if failingTest.TestName == playwrightStartupMarker {
		return r.runOneStartup(ctx, failingTest)
	}

	file, title, err := splitPlaywrightName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	cfg := failingTest.RunnerConfig
	if cfg.ConfigFile == "" {
		cfg = Config{Path: file, ConfigFile: playwrightConfigGuess(file)}
	}

	cwd, configArg, _ := playwrightPaths(cfg)

	grep := "^" + playwrightRegexMeta.ReplaceAllString(title, `\$0`) + "$"

	stdout, stderr, runErr := r.exec(ctx, cwd, "test", "--reporter=json",
		"--config", configArg, "--grep", grep, file)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("playwright rerun of %s failed: %w",
			failingTest.TestName, runErr)
	}

	report, parseErr := parsePlaywrightReport(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("playwright rerun report parse failed", "error", parseErr)

		return false, stdout.String(), nil
	}

	for _, failure := range playwrightReportToFailingTests(report, "", "") {
		if failure.TestName == failingTest.TestName {
			return false, failure.FailureOutput, nil
		}
	}

	return true, "", nil
}

// runOneStartup is the synthetic-marker branch of RunOne. Re-runs the
// whole Playwright suite (no --grep, no file filter) and reports passed
// iff the new run had no per-test failures AND the exec exited zero.
func (r *PlaywrightRunner) runOneStartup(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	cfg := failingTest.RunnerConfig
	cwd, configArg, pathArg := playwrightPaths(cfg)

	stdout, stderr, runErr := r.exec(ctx, cwd, "test", "--reporter=json",
		"--config", configArg, pathArg)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("playwright startup rerun failed: %w", runErr)
	}

	report, parseErr := parsePlaywrightReport(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("playwright startup rerun report parse failed", "error", parseErr)

		return false, stdout.String(), nil
	}

	rerunFailures := playwrightReportToFailingTests(report, "", "")
	if runErr == nil && len(rerunFailures) == 0 {
		return true, "", nil
	}

	output := stderr.String()
	if len(rerunFailures) > 0 {
		output = rerunFailures[0].FailureOutput
	}

	return false, TruncateTail(output, FailureOutputCap), nil
}

// playwrightConfigGuess infers the Playwright config path from a spec
// file path. Used by RunOne when only the file is known. Walks up from
// the spec's directory looking for a `playwright.config.ts` file on
// disk; falls back to the file's own directory if no marker is found.
func playwrightConfigGuess(specPath string) string {
	dir := filepath.Dir(specPath)
	for {
		candidate := filepath.Join(dir, "playwright.config.ts")

		_, statErr := os.Stat(candidate)
		if statErr == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return filepath.Join(filepath.Dir(specPath), "playwright.config.ts")
}

// playwrightPaths derives the (cwd, --config argument, positional path
// argument) triple from a Config. cwd is the directory containing the
// Playwright config (so `npx playwright` finds the local node_modules);
// the config and path args are made relative to cwd.
func playwrightPaths(cfg Config) (string, string, string) {
	cwd := "."
	if cfg.ConfigFile != "" {
		cwd = filepath.Dir(cfg.ConfigFile)
	}

	configArg := cfg.ConfigFile
	pathArg := cfg.Path

	if cwd == "." {
		return cwd, configArg, pathArg
	}

	relConfig, relConfigErr := filepath.Rel(cwd, cfg.ConfigFile)
	if relConfigErr == nil {
		configArg = relConfig
	}

	relPath, relPathErr := filepath.Rel(cwd, cfg.Path)
	if relPathErr == nil {
		pathArg = relPath
	}

	return cwd, configArg, pathArg
}

// exec runs `npx playwright ...` with the supplied args. cwd is set so
// `npx` resolves the local Playwright install. Non-zero exit codes are
// expected on test failure and returned without wrapping.
func (r *PlaywrightRunner) exec(
	ctx context.Context,
	cwd string,
	args ...string,
) (bytes.Buffer, bytes.Buffer, error) {
	allArgs := append([]string{"playwright"}, args...)
	cmd := exec.CommandContext(ctx, "npx", allArgs...)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		return stdout, stderr, fmt.Errorf("playwright exec: %w", runErr)
	}

	return stdout, stderr, nil
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

// parsePlaywrightReport decodes the single JSON document emitted by the
// `--reporter=json` reporter. Trims any leading log noise written by
// Playwright before the JSON document. The decoded shape lives in the
// `dto` package because it mirrors a third-party wire format (camelCase
// keys), which the linter excludes from tagliatelle there.
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

// playwrightReportToFailingTests walks the suite tree and collects one
// FailingTest per failed result, tagging each with the supplied service
// and layer. Free function rather than a method on dto.PlaywrightReport
// so the dto package stays passive.
func playwrightReportToFailingTests(
	report *dto.PlaywrightReport,
	service, layer string,
) []*FailingTest {
	collector := &playwrightCollector{service: service, layer: layer}
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
	layer   string
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
			ID:            BuildID(c.service, c.layer, FrameworkPlaywright, name),
			Service:       c.service,
			Layer:         c.layer,
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
