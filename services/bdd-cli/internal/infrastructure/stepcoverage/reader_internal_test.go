package stepcoverage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
)

// The two suites this repository declares, and the shape each one's
// report has: one that binds everything, one that does not.
const (
	// cliFile is the name a suite gives its own report: the suite, not
	// the command, so two suites answering one ask never collide.
	cliFile   = "bdd-cli.json"
	cliReport = `{"schema":1,"suite":"bdd-cli","examined":["E2E-001","E2E-002"],
		"unbound":[{"scenario":"E2E-002","step":"the relay restarts"}]}`
	webReport = `{"schema":1,"suite":"bdd-web","examined":["E2E-047"],"unbound":[]}`
)

// reportDirWith lays out a report directory the way a coverage command
// leaves one: one file per suite, named after it.
func reportDirWith(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	for name, body := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600)
		if err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return dir
}

// mergeDir reads a directory and folds it, which is the pair Ask runs.
func mergeDir(t *testing.T, dir string) (*Answer, error) {
	t.Helper()

	reports, err := readReports(dir)
	if err != nil {
		return nil, err
	}

	return merge(architecture.Testing{Framework: "go-test"},
		[]string{"bdd-cli", "bdd-web"}, reports, "")
}

// One command may start several suites, and the engine must end up
// holding every answer — the whole point of the report directory.
func TestMergeHoldsEverySuitesAnswer(t *testing.T) {
	t.Parallel()

	dir := reportDirWith(t, map[string]string{
		cliFile:        cliReport,
		"bdd-web.json": webReport,
	})

	answer, err := mergeDir(t, dir)
	if err != nil {
		t.Fatalf("merge two reports: %v", err)
	}

	for _, id := range []string{"E2E-001", "E2E-002", "E2E-047"} {
		if !answer.Examined[id] {
			t.Errorf("%s was examined by a suite but is missing from the merged answer", id)
		}
	}

	if len(answer.Gaps["E2E-002"]) != 1 {
		t.Errorf("E2E-002 gaps = %v, want the one step bdd-cli reported", answer.Gaps["E2E-002"])
	}
}

// Two reports for one suite disagree about a tree only one of them can
// have looked at, so neither may be picked.
func TestMergeRefusesTwoReportsForOneSuite(t *testing.T) {
	t.Parallel()

	dir := reportDirWith(t, map[string]string{
		cliFile:     cliReport,
		"copy.json": cliReport,
	})

	_, err := mergeDir(t, dir)
	if !errors.Is(err, ErrDuplicateSuiteReported) {
		t.Fatalf("err = %v, want ErrDuplicateSuiteReported", err)
	}
}

// An empty directory is "no suite answered", which is not "no suite has
// gaps" — walking on the second reading would report coverage nobody has.
func TestReadReportsRefusesAnEmptyDirectory(t *testing.T) {
	t.Parallel()

	_, err := readReports(t.TempDir())
	if !errors.Is(err, ErrNoReport) {
		t.Fatalf("err = %v, want ErrNoReport", err)
	}
}

// The suite is read from the report's own field, never from the file
// name, so a command pointed at the wrong package is still caught.
func TestMergeRefusesASuiteTheArchitectureDoesNotDeclare(t *testing.T) {
	t.Parallel()

	dir := reportDirWith(t, map[string]string{
		cliFile: `{"schema":1,"suite":"somebody-else","examined":["E2E-001"],"unbound":[]}`,
	})

	_, err := mergeDir(t, dir)
	if !errors.Is(err, ErrWrongSuiteReported) {
		t.Fatalf("err = %v, want ErrWrongSuiteReported", err)
	}
}

// A gap in a scenario the report never claims to have examined is half an
// answer, and merging half is how a partial answer becomes a silent pass.
func TestMergeRefusesAGapOutsideExamined(t *testing.T) {
	t.Parallel()

	dir := reportDirWith(t, map[string]string{
		cliFile: `{"schema":1,"suite":"bdd-cli","examined":["E2E-001"],
			"unbound":[{"scenario":"E2E-999","step":"the relay restarts"}]}`,
	})

	_, err := mergeDir(t, dir)
	if !errors.Is(err, ErrInconsistentReport) {
		t.Fatalf("err = %v, want ErrInconsistentReport", err)
	}
}
