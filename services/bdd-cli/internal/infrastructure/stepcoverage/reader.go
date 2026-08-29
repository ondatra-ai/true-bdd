// Package stepcoverage asks a test suite which of its scenarios have a
// step no definition binds.
//
// The question is a regexp match against the patterns a suite
// registered, and the suite is the only thing that holds those patterns
// after they are built — some are assembled at registration rather than
// written as literals, so reading the source can only ever approximate
// the answer. A check that approximates the runner is worse than no
// check: it makes the registry and the run disagree silently, which is
// the one failure this whole arrangement exists to prevent.
//
// So the suite is asked, through a command it declares. The engine stays
// framework-agnostic — it runs what the spec says and reads JSON back.
package stepcoverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// ReportEnv names the file the suite must write its report to — not
// stdout: a Go test's stdout carries the framework's own noise, so a
// report mixed into it would need a parser nobody can predict.
const ReportEnv = "TRUEBDD_COVERAGE_REPORT"

// ErrNoReport signals a coverage command that ran but wrote no report.
// "The suite could not answer" is not "the suite has no gaps", and only
// one of those is safe to walk on.
var ErrNoReport = errors.New("coverage command wrote no report")

// ErrAmbiguousStep signals a step matched by two definitions. Refused
// rather than reported as a gap: which one runs depends on registration
// order, and no fix turn should paper over that by adding a third.
var ErrAmbiguousStep = errors.New("step matches more than one definition")

// ErrUnknownSchema signals a report shape this engine does not know.
// Refused rather than read with today's meanings: a future schema could
// change what an empty `unbound` means, and guessing wrong drops scenarios silently.
var ErrUnknownSchema = errors.New("coverage report uses an unknown schema version")

// ErrSuiteCouldNotAnswer signals a suite that reported a failure of its
// own — an uncompilable step pattern, a registry that would not load.
var ErrSuiteCouldNotAnswer = errors.New("the suite could not report which steps bind")

// ErrInconsistentReport signals a report whose own fields disagree.
var ErrInconsistentReport = errors.New("coverage report is internally inconsistent")

// ErrWrongSuiteReported signals a report written by a suite other than
// the one asked — almost always a copy-pasted `coverage:` command. Acting
// on it would narrow this suite's walk on scenarios nobody examined.
var ErrWrongSuiteReported = errors.New("coverage report names a different suite")

// Gap is one scenario step that binds to no definition.
type Gap struct {
	Scenario string `json:"scenario"`
	Step     string `json:"step"`
}

// Ambiguity is one step matched by more than one definition.
type Ambiguity struct {
	Scenario string   `json:"scenario"`
	Step     string   `json:"step"`
	Patterns []string `json:"patterns"`
}

// Report is what a suite writes when asked which steps bind.
type Report struct {
	Schema int    `json:"schema"`
	Suite  string `json:"suite"`
	// Examined names every scenario the suite actually resolved. A
	// scenario absent from it was not looked at, whatever `Unbound`
	// says.
	Examined  []string    `json:"examined"`
	Unbound   []Gap       `json:"unbound"`
	Ambiguous []Ambiguity `json:"ambiguous"`
	// Failure is the suite reporting it could not answer at all.
	Failure string `json:"failure,omitempty"`
}

// Answer is one suite's reply: which scenarios it looked at, and which of
// them have a step that binds to nothing. The two are kept separate —
// see Report.Examined for why an empty gap list alone means nothing.
type Answer struct {
	Examined map[string]bool
	Gaps     map[string][]string
}

// commandTimeout bounds the coverage subprocess: generous, because a cold
// `go test` compiles the world; bounded, because a deadlocked suite would
// otherwise hang `build tests` before its first AI turn with no output at all.
const commandTimeout = 10 * time.Minute

// waitDelay bounds how long Wait blocks after the process is gone — same
// value and reason as cliWaitDelay in adapters/ai: a coverage command that
// leaves a grandchild holding stdout (a dev server, a browser driver) would otherwise hang `build tests` forever.
const waitDelay = 10 * time.Second

// schemaVersion is the report format this reader understands.
const schemaVersion = 1

// Validate checks every declared coverage command before the first one is
// spawned — same reason `build code` has a pre-pass: finding one unsplittable
// after two suites already compiled costs minutes for a verdict the spec could give immediately.
func Validate(suites []architecture.Suite) error {
	for _, suite := range suites {
		if strings.TrimSpace(suite.Commands.Coverage) == "" {
			continue
		}

		_, err := testrunner.SplitCommand(suite.Commands.Coverage)
		if err != nil {
			return fmt.Errorf("%s: commands.coverage: %w", suite.Label(), err)
		}
	}

	return nil
}

// Ask runs one suite's coverage command and returns what it examined and
// found. Exit status is ignored on purpose: a coverage test legitimately
// FAILS when it finds gaps, so the report file — not the status — is the answer.
func Ask(ctx context.Context, suite architecture.Suite, repoRoot string) (*Answer, error) {
	argv, err := testrunner.SplitCommand(suite.Commands.Coverage)
	if err != nil {
		return nil, fmt.Errorf("%s: commands.coverage: %w", suite.Label(), err)
	}

	reportPath, cleanup, err := reportFile(suite)
	if err != nil {
		return nil, err
	}

	defer cleanup()

	output, runErr := runCoverage(ctx, argv, commandDir(suite, repoRoot), reportPath)

	report, err := readReport(reportPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", suite.Label(), errors.Join(err, runErr), output)
	}

	err = checkReport(suite, report, output)
	if err != nil {
		return nil, err
	}

	answer := &Answer{
		Examined: make(map[string]bool, len(report.Examined)),
		Gaps:     map[string][]string{},
	}

	for _, id := range report.Examined {
		answer.Examined[id] = true
	}

	for _, gap := range report.Unbound {
		answer.Gaps[gap.Scenario] = append(answer.Gaps[gap.Scenario], gap.Step)
	}

	slog.Info("Asked a suite which steps bind",
		"suite", suite.Label(),
		"examined", len(answer.Examined),
		"scenarios_with_gaps", len(answer.Gaps),
	)

	return answer, nil
}

// checkReport refuses a report the engine must not act on.
func checkReport(suite architecture.Suite, report *Report, output string) error {
	if report.Schema != schemaVersion {
		return fmt.Errorf("%s: %w: got %d, want %d",
			suite.Label(), ErrUnknownSchema, report.Schema, schemaVersion)
	}

	if report.Failure != "" {
		return fmt.Errorf("%s: %w: %s", suite.Label(), ErrSuiteCouldNotAnswer, report.Failure)
	}

	// Checked, not merely decoded: a copy-pasted `coverage:` command runs the
	// wrong package and returns a well-formed report the rest of this function
	// can't tell is wrong. A blank `suite` is refused too, not exempted, so an unfilled field can't slip through.
	if report.Suite != suite.Name {
		return fmt.Errorf("%s: %w: the report says %q\n%s",
			suite.Label(), ErrWrongSuiteReported, report.Suite, output)
	}

	if len(report.Ambiguous) > 0 {
		first := report.Ambiguous[0]

		return fmt.Errorf("%s: %s: %q: %w (%s)",
			suite.Label(), first.Scenario, first.Step, ErrAmbiguousStep,
			strings.Join(first.Patterns, " | "))
	}

	// A report naming a gap in a scenario it never claims to have
	// examined is internally inconsistent, and acting on half of it is
	// how a partial answer becomes a silent pass.
	examined := make(map[string]bool, len(report.Examined))
	for _, id := range report.Examined {
		examined[id] = true
	}

	for _, gap := range report.Unbound {
		if !examined[gap.Scenario] {
			return fmt.Errorf("%s: %w: %s has a gap but is not in `examined`\n%s",
				suite.Label(), ErrInconsistentReport, gap.Scenario, output)
		}
	}

	return nil
}

// reportFile allocates the path the suite writes to, and the cleanup
// that removes it.
func reportFile(suite architecture.Suite) (string, func(), error) {
	dir, err := disk.TempDir("", "true-bdd-coverage-")
	if err != nil {
		return "", nil, fmt.Errorf("%s: create coverage report dir: %w", suite.Label(), err)
	}

	return filepath.Join(dir, "coverage.json"), func() { _ = disk.RemoveTree(dir) }, nil
}

// commandDir is where the coverage command runs: the directory holding
// the suite's `config:`, or the invocation dir when it declares none —
// same rule as testrunner.CommandDir, so all four `commands:` keys resolve identically.
func commandDir(suite architecture.Suite, repoRoot string) string {
	config := strings.TrimSpace(suite.ConfigFile)
	if config == "" {
		return repoRoot
	}

	dir := filepath.Dir(filepath.FromSlash(config))

	// Absolute stays absolute, exactly as testrunner.CommandDir leaves it:
	// filepath.Join(repoRoot, dir) would still produce `<repoRoot>/srv/app` for
	// an absolute `config: /srv/app/...`, landing this command in a directory that does not exist.
	if filepath.IsAbs(dir) {
		return dir
	}

	return filepath.Join(repoRoot, dir)
}

// runCoverage executes the command and returns its combined output for a
// diagnostic. Spawned in its own process group, and killed as a group —
// same reason as waitDelay above: a leaked grandchild would hold the pipes past a direct-child kill.
func runCoverage(ctx context.Context, argv []string, dir, reportPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	result, err := spec.Run(ctx, argv, cli.Options{
		Dir:       dir,
		Env:       cli.Inherit().Set(ReportEnv + "=" + reportPath),
		Output:    cli.Combined(),
		Group:     true,
		WaitDelay: waitDelay,
	})
	if err != nil {
		return result.Stdout, err
	}

	return result.Stdout, result.Err()
}

func readReport(path string) (*Report, error) {
	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoReport
	}

	if err != nil {
		return nil, fmt.Errorf("read coverage report: %w", err)
	}

	var report Report

	err = json.Unmarshal(data, &report)
	if err != nil {
		return nil, fmt.Errorf("parse coverage report: %w", err)
	}

	return &report, nil
}
