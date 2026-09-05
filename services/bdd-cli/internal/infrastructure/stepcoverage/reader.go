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
//
// One command, ANY NUMBER of suites: `go test ./tests/...` builds one
// binary per package, and each answers for the scenarios it owns. So the
// engine hands over a directory and merges every report it finds there,
// rather than a single file the last writer would erase the rest into.
package stepcoverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// ReportEnv names the directory every suite the command starts writes
// its report into, one file each — not stdout: a Go test's stdout
// carries framework noise no predictable parser survives. ADR 0011.
const ReportEnv = "TRUEBDD_COVERAGE_REPORT_DIR"

// ErrNoReport signals a coverage command that ran but left the report
// directory empty. "The suite could not answer" is not "the suite has no
// gaps", and only one of those is safe to walk on.
var ErrNoReport = errors.New("coverage command wrote no report")

// ErrDuplicateSuiteReported signals two reports claiming the same suite.
// Refused rather than merged: the two disagree about a suite only one of
// them can have looked at, and picking either is picking at random.
var ErrDuplicateSuiteReported = errors.New("two coverage reports name the same suite")

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
func Validate(testing architecture.Testing) error {
	if strings.TrimSpace(testing.Commands.Coverage) == "" {
		return nil
	}

	_, err := testrunner.SplitCommand(testing.Commands.Coverage)
	if err != nil {
		return fmt.Errorf("%s: commands.coverage: %w", testing.Label(), err)
	}

	return nil
}

// Ask runs the coverage command and returns what every suite it started
// examined and found. Exit status is ignored on purpose: a coverage test
// legitimately FAILS when it finds gaps, so the reports — not the status — are the answer.
func Ask(ctx context.Context, testing architecture.Testing, services []string, repoRoot string) (*Answer, error) {
	suite := testing

	argv, err := testrunner.SplitCommand(testing.Commands.Coverage)
	if err != nil {
		return nil, fmt.Errorf("%s: commands.coverage: %w", suite.Label(), err)
	}

	dir, cleanup, err := reportDir(suite)
	if err != nil {
		return nil, err
	}

	defer cleanup()

	output, runErr := runCoverage(argv, repoRoot, dir)

	reports, err := readReports(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", suite.Label(), errors.Join(err, runErr), output)
	}

	answer, err := merge(suite, services, reports, output)
	if err != nil {
		return nil, err
	}

	slog.Info("Asked the suites which steps bind",
		"command", suite.Label(),
		"suites", len(reports),
		"examined", len(answer.Examined),
		"scenarios_with_gaps", len(answer.Gaps),
	)

	return answer, nil
}

// merge folds every report into one answer, refusing before it folds
// anything a report the engine must not act on.
func merge(
	suite architecture.Testing,
	services []string,
	reports []namedReport,
	output string,
) (*Answer, error) {
	answer := &Answer{Examined: map[string]bool{}, Gaps: map[string][]string{}}
	from := map[string]string{}

	for _, named := range reports {
		where := fmt.Sprintf("%s: %s", suite.Label(), named.file)

		err := checkReport(where, services, named.report, output)
		if err != nil {
			return nil, err
		}

		first, duplicate := from[named.report.Suite]
		if duplicate {
			return nil, fmt.Errorf("%s: %w: %q, already reported by %s",
				where, ErrDuplicateSuiteReported, named.report.Suite, first)
		}

		from[named.report.Suite] = named.file

		for _, id := range named.report.Examined {
			answer.Examined[id] = true
		}

		for _, gap := range named.report.Unbound {
			answer.Gaps[gap.Scenario] = append(answer.Gaps[gap.Scenario], gap.Step)
		}
	}

	return answer, nil
}

// checkReport refuses a report the engine must not act on. `where` names
// the command and the report file, since one ask now yields several.
func checkReport(where string, services []string, report *Report, output string) error {
	if report.Schema != schemaVersion {
		return fmt.Errorf("%s: %w: got %d, want %d",
			where, ErrUnknownSchema, report.Schema, schemaVersion)
	}

	if report.Failure != "" {
		return fmt.Errorf("%s: %w: %s", where, ErrSuiteCouldNotAnswer, report.Failure)
	}

	// Checked, not merely decoded: a copy-pasted `coverage:` command runs the
	// wrong package and returns a well-formed report nothing else can tell is
	// wrong. No suite name survives, so the check is against the services.
	if !slices.Contains(services, report.Suite) {
		return fmt.Errorf("%s: %w: the report says %q\n%s",
			where, ErrWrongSuiteReported, report.Suite, output)
	}

	if len(report.Ambiguous) > 0 {
		first := report.Ambiguous[0]

		return fmt.Errorf("%s: %s: %q: %w (%s)",
			where, first.Scenario, first.Step, ErrAmbiguousStep,
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
				where, ErrInconsistentReport, gap.Scenario, output)
		}
	}

	return nil
}

// reportDir allocates the directory the suites write into, and the
// cleanup that removes it.
func reportDir(suite architecture.Testing) (string, func(), error) {
	dir, err := disk.TempDir("", "true-bdd-coverage-")
	if err != nil {
		return "", nil, fmt.Errorf("%s: create coverage report dir: %w", suite.Label(), err)
	}

	return dir, func() { _ = disk.RemoveTree(dir) }, nil
}

// namedReport is one suite's report and the file it arrived in, so a
// refusal can name the file a person has to open.
type namedReport struct {
	file   string
	report *Report
}

// readReports reads every report the command left behind. An empty
// directory is ErrNoReport: no suite answered, which is not the same as
// no suite having gaps.
func readReports(dir string) ([]namedReport, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list coverage reports: %w", err)
	}

	if len(matches) == 0 {
		return nil, ErrNoReport
	}

	reports := make([]namedReport, 0, len(matches))

	for _, path := range matches {
		report, readErr := readReport(path)
		if readErr != nil {
			return nil, readErr
		}

		reports = append(reports, namedReport{file: filepath.Base(path), report: report})
	}

	return reports, nil
}

// runCoverage executes the command and returns its combined output for a
// diagnostic. Spawned in its own process group, and killed as a group —
// same reason as waitDelay above: a leaked grandchild would hold the pipes past a direct-child kill.
func runCoverage(argv []string, dir, reportPath string) (string, error) {
	result, err := spec.Run(argv, cli.Options{
		Timeout:   commandTimeout,
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
