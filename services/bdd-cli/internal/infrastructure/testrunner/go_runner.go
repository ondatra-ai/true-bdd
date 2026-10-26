package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// goTestNameSeparator joins package + test in FailingTest.TestName so it
// round-trips through `-run '^<test>$' <package>` in RunOne. `::` was
// chosen because Go package paths contain `/` but never `::`.
const goTestNameSeparator = "::"

// goTestRunFlag is the flag that narrows a `go test` invocation to a
// single test. Appended to the suite's own command on a rerun.
const goTestRunFlag = "-run"

// goBuildFailureMarker is the synthetic test-name suffix used when a Go
// package fails to compile (no individual test failed because nothing
// could run). RunOne branches on this suffix to skip the `-run` filter.
const goBuildFailureMarker = "<build>"

// ErrInvalidGoTestName signals that a FailingTest.TestName intended for
// the Go runner is not in the expected "<package>::<func>" shape.
var ErrInvalidGoTestName = errors.New("invalid go-test TestName shape")

// GoTestRunner runs `go test -json` and parses its event stream into
// FailingTest values.
type GoTestRunner struct {
	artifacts *Artifacts
}

// NewGoTestRunner builds a GoTestRunner writing its captured output
// through artifacts, which may be nil to capture nothing.
func NewGoTestRunner(artifacts *Artifacts) *GoTestRunner {
	return &GoTestRunner{artifacts: artifacts}
}

// Discover runs the suite's declared command verbatim (build tags, package
// selectors, flags included — which tests exist is the host's statement,
// not the engine's guess), returning one FailingTest per failure or compile error.
func (r *GoTestRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, suite string,
) ([]*FailingTest, error) {
	argv, err := cfg.argv()
	if err != nil {
		return nil, fmt.Errorf("go-test command for %s/%s: %w", service, suite, err)
	}

	argv = appendGoScenarioFilter(argv, cfg.Tests)

	stdout, stderr, runErr := r.exec(CommandDir(cfg), PhaseDiscover, argv)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("go test discovery failed under %s: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	events := parseGoTestEvents(stdout.Bytes())

	failures := events.toFailingTests(service, suite)
	if len(cfg.Tests) > 0 && len(failures) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("selected scenarios failed: %w: %s", runErr, stderr.String())
		}

		err := events.requireTests(cfg.Tests)
		if err != nil {
			return nil, err
		}
	}

	for _, failure := range failures {
		failure.RunnerConfig = cfg
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].ID < failures[j].ID })

	return failures, nil
}

// RunOne re-executes a single failing Go test under the same command
// Discover used: a rerun assembled from scratch would compile a different
// set of packages and might never reproduce the failure. Build-failure entries (`::<build>`) rerun unfiltered.
func (r *GoTestRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	pkg, test, err := splitGoTestName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	argv, err := commandArgv(failingTest)
	if err != nil {
		return false, "", err
	}

	args := appendGoRunFilter(argv, test)

	stdout, stderr, runErr := r.exec(CommandDir(failingTest.RunnerConfig), PhaseRerun, args)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("go test rerun of %s failed: %w",
			failingTest.TestName, runErr)
	}

	events := parseGoTestEvents(stdout.Bytes())

	passed := !events.hasFailureFor(pkg, test)
	if test == goBuildFailureMarker {
		passed = events.buildPassed(pkg)
	}

	// A package that now compiles and passes has no failure to show, as a
	// Jest or Playwright pass has none; its `ok` line's elapsed time varies.
	if passed && test == goBuildFailureMarker {
		return true, "", nil
	}

	return passed, events.outputFor(pkg, test), nil
}

// exec runs the suite's command with the supplied argv and captures
// stdout/stderr. phase labels the invocation in log/filename. `go test`
// exits non-zero on test failure — that is not an infrastructure error.
func (r *GoTestRunner) exec(
	cwd, phase string,
	argv []string,
) (bytes.Buffer, bytes.Buffer, error) {
	// Empty cwd means "inherit the engine's own working directory",
	// which is what a suite declaring no `config:` file asks for.
	return runLogged(argv, cwd, spawnMeta{
		binary:    argv[0],
		args:      argv[1:],
		framework: FrameworkGoTest,
		phase:     phase,
		artifacts: r.artifacts,
	})
}

// appendGoRunFilter narrows the suite's command to a single test. Appended,
// not inserted: `go test` takes the LAST `-run` in Go's flag parsing, so a
// spec-declared `-run` would win over an inserted filter — appending also can't land in front of a package selector.
func appendGoRunFilter(base []string, test string) []string {
	if test == goBuildFailureMarker {
		return base
	}

	filter := []string{goTestRunFlag, buildGoRunFilter(test)}

	args := make([]string, 0, len(base)+len(filter))
	args = append(args, base...)

	return append(args, filter...)
}

// buildGoRunFilter assembles the `-run` regex for one (possibly nested)
// test name. Top-level `TestFoo` becomes `^TestFoo$`; subtest
// `TestFoo/sub` becomes `^TestFoo$/^sub$`.
func buildGoRunFilter(test string) string {
	parts := strings.Split(test, "/")
	for i, part := range parts {
		parts[i] = "^" + part + "$"
	}

	return strings.Join(parts, "/")
}

// splitGoTestName parses a "<package>::<func>" TestName, returning
// ErrInvalidGoTestName if the shape is malformed.
func splitGoTestName(name string) (string, string, error) {
	pkg, test, ok := strings.Cut(name, goTestNameSeparator)
	if !ok || pkg == "" || test == "" {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidGoTestName, name)
	}

	return pkg, test, nil
}

// goEventStream aggregates a `go test -json` run per (package, test);
// the empty test holds package-level events. Compiler lines are keyed
// by import path, joined to a package through its fail's FailedBuild.
type goEventStream struct {
	testPassed    map[string]bool
	testFailed    map[string]bool
	packageFailed map[string]bool
	packageBuilt  map[string]bool
	testOutput    map[string]*strings.Builder
	packageOutput map[string]*strings.Builder
	buildOutput   map[string]*strings.Builder
	failedBuild   map[string]string
}

func newGoEventStream() *goEventStream {
	return &goEventStream{
		testPassed:    make(map[string]bool),
		testFailed:    make(map[string]bool),
		packageFailed: make(map[string]bool),
		packageBuilt:  make(map[string]bool),
		testOutput:    make(map[string]*strings.Builder),
		packageOutput: make(map[string]*strings.Builder),
		buildOutput:   make(map[string]*strings.Builder),
		failedBuild:   make(map[string]string),
	}
}

// parseGoTestEvents decodes the line-delimited JSON event stream from
// `go test -json`, retaining complete events and skipping malformed lines.
func parseGoTestEvents(payload []byte) *goEventStream {
	stream := newGoEventStream()

	for _, line := range bytes.Split(payload, []byte("\n")) {
		var event dto.GoTestEvent

		err := json.Unmarshal(line, &event)
		if err != nil {
			continue
		}

		stream.absorb(event)
	}

	return stream
}

// absorb integrates one event into the stream's per-test/per-package
// aggregates.
func (s *goEventStream) absorb(event dto.GoTestEvent) {
	key := event.Package + "\x00" + event.Test

	if event.Action == "build-output" {
		appendBuilder(s.buildOutput, event.ImportPath, event.Output)
	}

	if event.FailedBuild != "" {
		s.failedBuild[event.Package] = event.FailedBuild
	}

	if event.Action == "output" {
		if event.Test != "" {
			appendBuilder(s.testOutput, key, event.Output)
		} else {
			appendBuilder(s.packageOutput, event.Package, event.Output)
		}
	}

	s.absorbResult(event)
}

func (s *goEventStream) absorbResult(event dto.GoTestEvent) {
	key := event.Package + "\x00" + event.Test

	if event.Action == "pass" {
		if event.Test == "" {
			s.packageBuilt[event.Package] = true
		} else {
			s.testPassed[key] = true
		}
	}

	// Go emits package-level skip for packages with no test files.
	if event.Action == "skip" && event.Test == "" {
		s.packageBuilt[event.Package] = true
	}

	if event.Action == "fail" {
		if event.Test != "" {
			s.testFailed[key] = true
		} else {
			s.packageFailed[event.Package] = true
		}
	}
}

// appendBuilder appends to a builder under `key`, creating it on first
// touch.
func appendBuilder(builders map[string]*strings.Builder, key, line string) {
	builder, ok := builders[key]
	if !ok {
		builder = &strings.Builder{}
		builders[key] = builder
	}

	builder.WriteString(line)
}

// hasFailureFor reports whether the supplied (package, test) pair has a
// fail event in the stream.
func (s *goEventStream) hasFailureFor(pkg, test string) bool {
	if test == goBuildFailureMarker {
		return s.packageFailed[pkg]
	}

	return s.testFailed[pkg+"\x00"+test]
}

// buildPassed requires the package's own successful result, without any failure.
func (s *goEventStream) buildPassed(pkg string) bool {
	return s.packageBuilt[pkg] && !s.packageFailed[pkg] && s.failedBuild[pkg] == "" &&
		!s.packageHasExplainedFailure(pkg)
}

// outputFor returns the complete output captured for a
// (package, test) pair. A package's entry leads with the compiler's
// lines for the build it failed on, which say why it did not compile.
func (s *goEventStream) outputFor(pkg, test string) string {
	if test == goBuildFailureMarker {
		var output strings.Builder

		if builder, ok := s.buildOutput[s.failedBuild[pkg]]; ok {
			output.WriteString(builder.String())
		}

		if builder, ok := s.packageOutput[pkg]; ok {
			output.WriteString(builder.String())
		}

		return output.String()
	}

	if builder, ok := s.testOutput[pkg+"\x00"+test]; ok {
		return builder.String()
	}

	return ""
}

// toFailingTests projects the stream into FailingTest values tagged with
// service + suite. Per-test failures take precedence over package-level
// ones; a package already explained by per-test entries is omitted.
func (s *goEventStream) toFailingTests(service, suite string) []*FailingTest {
	out := make([]*FailingTest, 0, len(s.testFailed)+len(s.packageFailed))

	for key := range s.testFailed {
		pkg, test, _ := strings.Cut(key, "\x00")
		out = append(out, s.testToFailingTest(service, suite, pkg, test))
	}

	for pkg := range s.packageFailed {
		if s.packageHasExplainedFailure(pkg) {
			continue
		}

		out = append(out, s.packageToFailingTest(service, suite, pkg))
	}

	return out
}

// packageHasExplainedFailure reports whether a per-test failure already
// exists under pkg — used by toFailingTests to avoid double-counting a
// package-level fail summary when individual tests already explain it.
func (s *goEventStream) packageHasExplainedFailure(pkg string) bool {
	prefix := pkg + "\x00"
	for key := range s.testFailed {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}

// testToFailingTest builds one FailingTest from a per-test failure.
func (s *goEventStream) testToFailingTest(service, suite, pkg, test string) *FailingTest {
	name := pkg + goTestNameSeparator + test

	return &FailingTest{
		ID:            BuildID(service, suite, FrameworkGoTest, name),
		Service:       service,
		Suite:         suite,
		Framework:     FrameworkGoTest,
		TestName:      name,
		FilePath:      pkg,
		FailureOutput: s.outputFor(pkg, test),
	}
}

// packageToFailingTest builds the synthetic FailingTest used when a
// package failed to compile or otherwise produced no per-test failures.
func (s *goEventStream) packageToFailingTest(service, suite, pkg string) *FailingTest {
	name := pkg + goTestNameSeparator + goBuildFailureMarker

	return &FailingTest{
		ID:            BuildID(service, suite, FrameworkGoTest, name),
		Service:       service,
		Suite:         suite,
		Framework:     FrameworkGoTest,
		TestName:      name,
		FilePath:      pkg,
		FailureOutput: s.outputFor(pkg, goBuildFailureMarker),
	}
}
