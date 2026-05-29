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

	"bdd-cli/src/internal/infrastructure/testrunner/dto"
)

// goTestNameSeparator joins package + test in FailingTest.TestName so the
// resulting id round-trips through `-run '^<test>$' <package>` in RunOne.
// `::` was chosen because Go package paths legitimately contain `/` but
// never `::`.
const goTestNameSeparator = "::"

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
// FailingTest values. Stateless; constructed once per process.
type GoTestRunner struct{}

// NewGoTestRunner builds a GoTestRunner.
func NewGoTestRunner() *GoTestRunner {
	return &GoTestRunner{}
}

// Discover runs `go test -C cfg.Path -json -count=1 ./...` and returns
// one FailingTest per failed test plus one synthetic entry per
// non-test-bearing package failure (typically compile errors).
func (r *GoTestRunner) Discover(
	ctx context.Context,
	cfg Config,
	service, layer string,
) ([]*FailingTest, error) {
	stdout, stderr, runErr := r.exec(ctx, "-C", cfg.Path, "-json", "-count=1", "./...")
	if runErr != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("go test discovery failed under %s: %w (stderr: %s)",
			cfg.Path, runErr, stderr.String())
	}

	events, parseErr := parseGoTestEvents(stdout.Bytes())
	if parseErr != nil {
		slog.Warn("go test JSON stream had unparseable lines", "error", parseErr)
	}

	failures := events.toFailingTests(service, layer)

	for _, failure := range failures {
		failure.RunnerConfig = cfg
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].ID < failures[j].ID })

	return failures, nil
}

// RunOne re-executes a single failing Go test by its TestName id. For
// build-failure synthetic entries (TestName suffix `::<build>`) it
// instead re-runs the whole package and looks for any remaining failure.
func (r *GoTestRunner) RunOne(
	ctx context.Context,
	failingTest *FailingTest,
) (bool, string, error) {
	pkg, test, err := splitGoTestName(failingTest.TestName)
	if err != nil {
		return false, "", err
	}

	args := buildGoRunOneArgs(pkg, test)

	stdout, stderr, runErr := r.exec(ctx, args...)
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

// exec runs `go test` with the supplied args and captures stdout/stderr.
// `go test` exits non-zero on test failure — that is not an
// infrastructure error and is returned to the caller without wrapping.
func (r *GoTestRunner) exec(
	ctx context.Context,
	args ...string,
) (bytes.Buffer, bytes.Buffer, error) {
	allArgs := append([]string{"test"}, args...)
	cmd := exec.CommandContext(ctx, "go", allArgs...)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		return stdout, stderr, fmt.Errorf("go test exec: %w", runErr)
	}

	return stdout, stderr, nil
}

// buildGoRunOneArgs assembles the `go test` invocation for re-running a
// single test. Build-failure synthetic entries use no `-run` filter so
// the package's compile is the verdict.
func buildGoRunOneArgs(pkg, test string) []string {
	if test == goBuildFailureMarker {
		return []string{"-json", "-count=1", pkg}
	}

	runFilter := buildGoRunFilter(test)

	return []string{"-json", "-count=1", "-run", runFilter, pkg}
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

// toFailingTests projects the stream into a slice of FailingTest values
// tagged with the supplied service + layer. Per-test failures take
// precedence over package-level failures; a package whose failure is
// already explained by per-test entries is omitted from the output.
func (s *goEventStream) toFailingTests(service, layer string) []*FailingTest {
	out := make([]*FailingTest, 0, len(s.testFailed)+len(s.packageFailed))

	for key := range s.testFailed {
		pkg, test, _ := strings.Cut(key, "\x00")
		out = append(out, s.testToFailingTest(service, layer, pkg, test))
	}

	for pkg := range s.packageFailed {
		if s.packageHasExplainedFailure(pkg) {
			continue
		}

		out = append(out, s.packageToFailingTest(service, layer, pkg))
	}

	return out
}

// packageHasExplainedFailure reports whether at least one per-test
// failure already exists under the supplied package. Used to avoid
// double-counting a package-level fail summary when individual tests
// failed.
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
func (s *goEventStream) testToFailingTest(service, layer, pkg, test string) *FailingTest {
	name := pkg + goTestNameSeparator + test

	return &FailingTest{
		ID:            BuildID(service, layer, FrameworkGoTest, name),
		Service:       service,
		Layer:         layer,
		Framework:     FrameworkGoTest,
		TestName:      name,
		FilePath:      pkg,
		FailureOutput: s.outputFor(pkg, test),
	}
}

// packageToFailingTest builds the synthetic FailingTest used when a
// package failed to compile or otherwise produced no per-test failures.
func (s *goEventStream) packageToFailingTest(service, layer, pkg string) *FailingTest {
	name := pkg + goTestNameSeparator + goBuildFailureMarker

	return &FailingTest{
		ID:            BuildID(service, layer, FrameworkGoTest, name),
		Service:       service,
		Layer:         layer,
		Framework:     FrameworkGoTest,
		TestName:      name,
		FilePath:      pkg,
		FailureOutput: s.outputFor(pkg, goBuildFailureMarker),
	}
}
