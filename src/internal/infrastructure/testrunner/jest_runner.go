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

// jestNameSeparator joins the spec file path with the assertion
// fullName inside FailingTest.TestName so the (file, fullName) pair
// survives round-tripping through RunOne.
const jestNameSeparator = "::"

// ErrInvalidJestName signals that a FailingTest.TestName intended for
// the Jest runner is not in the expected "<file>::<fullName>" shape.
var ErrInvalidJestName = errors.New("invalid jest TestName shape")

// jestRegexMeta enumerates the regex meta-characters that must be
// escaped when building a `--testNamePattern` filter from a free-form
// assertion fullName.
var jestRegexMeta = regexp.MustCompile(`[\\^$.|?*+()[\]{}]`)

// JestRunner runs `npx jest --json` and decodes its trailing JSON
// document into FailingTest values.
type JestRunner struct{}

// NewJestRunner builds a JestRunner.
func NewJestRunner() *JestRunner {
	return &JestRunner{}
}

// Discover runs the full Jest suite declared by cfg and returns one
// FailingTest per failed assertion.
func (r *JestRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, layer string,
) ([]*FailingTest, error) {
	cwd, configArg, pathArg := jestPaths(cfg)

	stdout, stderr, runErr := r.exec(ctx, cwd, "--json",
		"--config", configArg, pathArg)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("jest discovery under %s failed: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	report, parseErr := parseJestReport(stdout.Bytes())
	if parseErr != nil {
		return nil, fmt.Errorf("jest report parse failed: %w", parseErr)
	}

	failures := jestReportToFailingTests(report, service, layer, cwd)

	for _, failure := range failures {
		failure.RunnerConfig = cfg
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].ID < failures[j].ID })

	return failures, nil
}

// RunOne re-executes one failing Jest assertion via
// `--testNamePattern` (anchored to the escaped fullName) plus the spec
// file positional argument.
func (r *JestRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	file, fullName, err := splitJestName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	cfg := failingTest.RunnerConfig
	if cfg.ConfigFile == "" {
		cfg = Config{Path: file, ConfigFile: jestConfigGuess(file)}
	}

	cwd, configArg, _ := jestPaths(cfg)
	pattern := "^" + jestRegexMeta.ReplaceAllString(fullName, `\$0`) + "$"

	stdout, stderr, runErr := r.exec(ctx, cwd, "--json",
		"--config", configArg, "--testNamePattern", pattern, file)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("jest rerun of %s failed: %w",
			failingTest.TestName, runErr)
	}

	report, parseErr := parseJestReport(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("jest rerun report parse failed", "error", parseErr)

		return false, stdout.String(), nil
	}

	for _, failure := range jestReportToFailingTests(report, "", "", cwd) {
		if failure.TestName == failingTest.TestName {
			return false, failure.FailureOutput, nil
		}
	}

	return true, "", nil
}

// exec runs `npx jest ...` with the supplied args. cwd is set so `npx`
// resolves the local Jest install. Non-zero exit codes are expected on
// test failure and returned without wrapping.
func (r *JestRunner) exec(
	ctx context.Context,
	cwd string,
	args ...string,
) (bytes.Buffer, bytes.Buffer, error) {
	allArgs := append([]string{"jest"}, args...)
	cmd := exec.CommandContext(ctx, "npx", allArgs...)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		return stdout, stderr, fmt.Errorf("jest exec: %w", runErr)
	}

	return stdout, stderr, nil
}

// jestPaths derives the (cwd, --config arg, positional path arg) triple
// from a Config. cwd is the directory containing the Jest config so
// `npx jest` resolves the local install; arguments are made relative.
func jestPaths(cfg Config) (string, string, string) {
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

// jestConfigGuess infers the Jest config path from a spec file. Walks
// up looking for the closest jest.config.{js,cjs,mjs,ts,json}. Falls
// back to the file's directory if no marker is found.
func jestConfigGuess(specPath string) string {
	candidates := []string{
		"jest.config.js", "jest.config.cjs", "jest.config.mjs",
		"jest.config.ts", "jest.config.json",
	}

	dir := filepath.Dir(specPath)

	for {
		for _, name := range candidates {
			candidate := filepath.Join(dir, name)

			_, statErr := os.Stat(candidate)
			if statErr == nil {
				return candidate
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return filepath.Join(filepath.Dir(specPath), "jest.config.js")
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

// parseJestReport decodes the JSON document on stdout. Jest writes only
// the JSON on stdout when `--json` is set, so no log-noise stripping is
// required; falls back to nil report on empty payload. The decoded
// shape lives in the `dto` package because it mirrors a third-party
// wire format (camelCase keys), which the linter excludes from
// tagliatelle there.
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

// jestReportToFailingTests walks the testResults tree and collects one
// FailingTest per failed assertion, tagging each with the supplied
// service and layer. The file path is rebased on cwd to keep it
// repo-relative for prompt display. Free function rather than a method
// on dto.JestReport so the dto package stays passive.
func jestReportToFailingTests(
	report *dto.JestReport,
	service, layer, cwd string,
) []*FailingTest {
	out := make([]*FailingTest, 0)

	for _, testResult := range report.TestResults {
		filePath := jestRepoRelative(cwd, testResult.Name)

		for _, assertion := range testResult.AssertionResults {
			if assertion.Status != statusFailed {
				continue
			}

			name := filePath + jestNameSeparator + assertion.FullName
			out = append(out, &FailingTest{
				ID:            BuildID(service, layer, FrameworkJest, name),
				Service:       service,
				Layer:         layer,
				Framework:     FrameworkJest,
				TestName:      name,
				FilePath:      filePath,
				FailureOutput: TruncateTail(strings.Join(assertion.FailureMessages, "\n"), FailureOutputCap),
			})
		}
	}

	return out
}

// jestRepoRelative rebases the absolute path Jest emits into a path
// relative to the supplied cwd, falling back to the absolute path if
// the relative computation fails.
func jestRepoRelative(cwd, absolutePath string) string {
	if !filepath.IsAbs(absolutePath) {
		return absolutePath
	}

	rel, err := filepath.Rel(cwd, absolutePath)
	if err != nil {
		return absolutePath
	}

	return rel
}
