package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// jestNameSeparator joins the spec file path with the assertion
// fullName inside FailingTest.TestName so the (file, fullName) pair
// survives round-tripping through RunOne.
const jestNameSeparator = "::"

const jestLoadMarker = "<load>"

// ErrInvalidJestName signals that a FailingTest.TestName intended for
// the Jest runner is not in the expected "<file>::<fullName>" shape.
var ErrInvalidJestName = errors.New("invalid jest TestName shape")

// jestRegexMeta enumerates the regex meta-characters that must be
// escaped when building a `--testNamePattern` filter from a free-form
// assertion fullName.
var jestRegexMeta = regexp.MustCompile(`[\\^$.|?*+()[\]{}]`)

// JestRunner runs `npx jest --json` and decodes its trailing JSON
// document into FailingTest values.
type JestRunner struct {
	artifacts *Artifacts
}

// NewJestRunner builds a JestRunner writing its captured output through
// artifacts, which may be nil to capture nothing.
func NewJestRunner(artifacts *Artifacts) *JestRunner {
	return &JestRunner{artifacts: artifacts}
}

// Discover runs the full Jest suite declared by cfg and returns one
// FailingTest per failed assertion.
func (r *JestRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, suite string,
) ([]*FailingTest, error) {
	argv, err := cfg.argv()
	if err != nil {
		return nil, fmt.Errorf("jest command for %s/%s: %w", service, suite, err)
	}

	cwd := CommandDir(cfg)

	stdout, stderr, runErr := r.exec(cwd, PhaseDiscover, argv)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("jest discovery under %s failed: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	report, parseErr := parseJestReport(stdout.Bytes())
	if parseErr != nil {
		return nil, fmt.Errorf("jest report parse failed: %w", errors.Join(parseErr, runErr))
	}

	failures := jestReportToFailingTests(report, service, suite)
	if runErr != nil && len(failures) == 0 {
		return nil, runErr
	}

	for _, failure := range failures {
		failure.RunnerConfig = cfg
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].ID < failures[j].ID })

	return failures, nil
}

// RunOne re-executes one failing Jest assertion via `--testNamePattern`
// (ANDed with the paths) appended to the suite's command. Never appends the
// spec file itself: Jest OR-s bare positionals, so an added file would widen the rerun instead of isolating it.
func (r *JestRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	_, fullName, err := splitJestName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	if fullName == jestLoadMarker {
		return r.runOneLoad(ctx, failingTest)
	}

	argv, err := commandArgv(failingTest)
	if err != nil {
		return false, "", err
	}

	cwd := CommandDir(failingTest.RunnerConfig)
	pattern := "^" + jestRegexMeta.ReplaceAllString(fullName, `\$0`) + "$"
	argv = append(argv, "--testNamePattern", pattern)

	stdout, _, runErr := r.exec(cwd, PhaseRerun, argv)

	report, parseErr := parseJestReport(stdout.Bytes())
	if parseErr != nil {
		return false, stdout.String(), errors.Join(parseErr, runErr)
	}

	if runErr != nil && len(jestReportToFailingTests(report, "", "")) == 0 {
		return false, stdout.String(), runErr
	}

	if jestRanNothing(report) {
		return false, stdout.String(), fmt.Errorf("%w: jest --testNamePattern %q matched no test for %s",
			ErrRerunSelectedNoTests, pattern, failingTest.TestName)
	}

	for _, failure := range jestReportToFailingTests(report, "", "") {
		if failure.TestName == failingTest.TestName {
			return false, failure.FailureOutput, nil
		}
	}

	return true, "", nil
}

// jestRanNothing reports whether the rerun executed no assertion at all:
// a pattern matching nothing produces a report with no failures — the
// same shape as a passing test — so without this a mistyped filter reads as a successful fix.
func jestRanNothing(report *dto.JestReport) bool {
	for _, testResult := range report.TestResults {
		if len(testResult.AssertionResults) > 0 {
			return false
		}
	}

	return true
}

// exec runs the suite's command with the supplied argv. cwd is the config
// directory so `npx` resolves the local Jest install (see CommandDir).
// phase labels the invocation in log/filename; non-zero exit is expected on test failure.
func (r *JestRunner) exec(
	cwd, phase string,
	argv []string,
) (bytes.Buffer, bytes.Buffer, error) {
	return runLogged(argv, cwd, spawnMeta{
		binary:    argv[0],
		args:      argv[1:],
		framework: FrameworkJest,
		phase:     phase,
		artifacts: r.artifacts,
	})
}

// splitJestName parses "<file>::<fullName>" into its parts, returning
// ErrInvalidJestName for malformed inputs.
func splitJestName(name string) (string, string, error) {
	file, fullName, ok := strings.Cut(name, jestNameSeparator)
	if !ok || file == "" || fullName == "" {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidJestName, name)
	}

	return file, fullName, nil
}

// parseJestReport decodes the JSON document on stdout: Jest with `--json`
// writes ONLY the JSON there (no log-noise stripping needed), falling back
// to an empty report on an empty payload. Lives in `dto` since its camelCase mirrors a third-party wire format.
func parseJestReport(payload []byte) (*dto.JestReport, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return &dto.JestReport{}, nil
	}

	start := bytes.IndexByte(trimmed, '{')
	if start < 0 {
		return &dto.JestReport{}, nil
	}

	var report dto.JestReport

	err := json.Unmarshal(trimmed[start:], &report)
	if err != nil {
		return nil, fmt.Errorf("decode jest json: %w", err)
	}

	return &report, nil
}

// jestReportToFailingTests walks testResults and collects one FailingTest
// per failed assertion, tagged with service/suite; the file path is made
// repo-relative (see jestRepoRelative). Free function so dto stays passive.
func jestReportToFailingTests(
	report *dto.JestReport,
	service, suite string,
) []*FailingTest {
	out := make([]*FailingTest, 0)
	loadFiles := make(map[string]bool)
	root := workingDir()

	for _, testResult := range report.TestResults {
		filePath := jestRepoRelative(root, testResult.Name)

		if testResult.Status == statusFailed && len(testResult.AssertionResults) == 0 && !loadFiles[filePath] {
			loadFiles[filePath] = true
			name := filePath + jestNameSeparator + jestLoadMarker
			out = append(out, &FailingTest{
				ID:      BuildID(service, suite, FrameworkJest, name),
				Service: service, Suite: suite, Framework: FrameworkJest,
				TestName: name, FilePath: filePath, FailureOutput: testResult.Message,
			})
		}

		for _, assertion := range testResult.AssertionResults {
			if assertion.Status != statusFailed {
				continue
			}

			name := filePath + jestNameSeparator + assertion.FullName
			out = append(out, &FailingTest{
				ID:            BuildID(service, suite, FrameworkJest, name),
				Service:       service,
				Suite:         suite,
				Framework:     FrameworkJest,
				TestName:      name,
				FilePath:      filePath,
				FailureOutput: strings.Join(assertion.FailureMessages, "\n"),
			})
		}
	}

	return out
}

// jestRepoRelative rebases Jest's absolute path onto root, the engine's own
// directory: rebased on the relative config dir it stayed absolute, so the
// test id carried the checkout's path and changed with it (#167).
func jestRepoRelative(root, absolutePath string) string {
	if !filepath.IsAbs(absolutePath) || root == "" {
		return absolutePath
	}

	rel, err := filepath.Rel(root, absolutePath)
	if err != nil {
		return absolutePath
	}

	return rel
}
