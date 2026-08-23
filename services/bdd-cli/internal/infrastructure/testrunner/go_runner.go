package testrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
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

const goBufferInitialCap = 1024
const goBufferMaxCap = 4 * 1024 * 1024

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
	argv, err := SplitCommand(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("go-test command for %s/%s: %w", service, suite, err)
	}

	stdout, stderr, runErr := r.exec(ctx, CommandDir(cfg), PhaseDiscover, argv)
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("go test discovery failed under %s: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	events, parseErr := parseGoTestEvents(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("go test JSON stream had unparseable lines", "error", parseErr)
	}

	failures := events.toFailingTests(service, suite)

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

	stdout, stderr, runErr := r.exec(ctx, CommandDir(failingTest.RunnerConfig), PhaseRerun, args)
	if runErr != nil && stdout.Len() == 0 {
		return false, stderr.String(), fmt.Errorf("go test rerun of %s failed: %w",
			failingTest.TestName, runErr)
	}

	events, parseErr := parseGoTestEvents(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("go test rerun JSON had unparseable lines", "error", parseErr)
	}

	passed := !events.hasFailureFor(pkg, test)
	output := events.outputFor(pkg, test)

	return passed, output, nil
}

// exec runs the suite's command with the supplied argv and captures
// stdout/stderr. phase labels the invocation in log/filename. `go test`
// exits non-zero on test failure — that is not an infrastructure error.
func (r *GoTestRunner) exec(
	ctx context.Context,
	cwd, phase string,
	argv []string,
) (bytes.Buffer, bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// Empty cwd means "inherit the engine's own working directory",
	// which is what a suite declaring no `config:` file asks for.
	cmd.Dir = cwd

	return runLogged(cmd, spawnMeta{
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

// goEventStream is the in-memory aggregation of a `go test -json` run.
// One key per (package, test) pair; the empty-test entry holds
// package-level events (build output, summary fail).
type goEventStream struct {
	testFailed    map[string]bool
	packageFailed map[string]bool
	testOutput    map[string]*strings.Builder
	packageOutput map[string]*strings.Builder
}

func newGoEventStream() *goEventStream {
	return &goEventStream{
		testFailed:    make(map[string]bool),
		packageFailed: make(map[string]bool),
		testOutput:    make(map[string]*strings.Builder),
		packageOutput: make(map[string]*strings.Builder),
	}
}

// parseGoTestEvents decodes the line-delimited JSON event stream from
// `go test -json`. Malformed lines are skipped with a logged warning;
// the function returns nil error unless the entire payload is unparseable.
func parseGoTestEvents(payload []byte) (*goEventStream, error) {
	stream := newGoEventStream()
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, goBufferInitialCap), goBufferMaxCap)

	for scanner.Scan() {
		var event dto.GoTestEvent

		err := json.Unmarshal(scanner.Bytes(), &event)
		if err != nil {
			continue
		}

		stream.absorb(event)
	}

	err := scanner.Err()
	if err != nil {
		return stream, fmt.Errorf("go test event scanner failed: %w", err)
	}

	return stream, nil
}

// absorb integrates one event into the stream's per-test/per-package
// aggregates.
func (s *goEventStream) absorb(event dto.GoTestEvent) {
	key := event.Package + "\x00" + event.Test

	if event.Action == "output" {
		if event.Test != "" {
			appendBuilder(s.testOutput, key, event.Output)
		} else {
			appendBuilder(s.packageOutput, event.Package, event.Output)
		}
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

// outputFor returns the tail-truncated output captured for a
// (package, test) pair.
func (s *goEventStream) outputFor(pkg, test string) string {
	if test == goBuildFailureMarker {
		if builder, ok := s.packageOutput[pkg]; ok {
			return TruncateTail(builder.String(), FailureOutputCap)
		}

		return ""
	}

	if builder, ok := s.testOutput[pkg+"\x00"+test]; ok {
		return TruncateTail(builder.String(), FailureOutputCap)
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
