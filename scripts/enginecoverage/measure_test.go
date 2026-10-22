package enginecoverage_test

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/scripts/enginecoverage"
)

// errSuiteRed stands in for a suite that failed under the covered binary.
var errSuiteRed = errors.New("suite red")

func samplePlan(t *testing.T, exercise func(binPath string, env []string) error) enginecoverage.Plan {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("testdata", "sample"))
	if err != nil {
		t.Fatalf("resolve the sample module: %v", err)
	}

	return enginecoverage.Plan{Root: root, Main: ".", Packages: "./...", Exercise: exercise}
}

// greetOnce runs the covered sample once, with a name to greet.
func greetOnce(binPath string, env []string) error {
	result, err := spec.Run([]string{binPath, "x"}, cli.Options{Env: cli.Inherit().Set(env...)})
	if err != nil {
		return err
	}

	return result.Err()
}

// Asserted structurally: where a block starts and how many statements it
// holds is the toolchain's choice and moved between Go 1.25 and 1.27, while
// which code ran and on which lines its unrun statements sit did not.
func TestMeasureFindsEveryFunctionTheExerciseLeftUnrun(t *testing.T) {
	t.Parallel()

	report, err := enginecoverage.Measure(samplePlan(t, greetOnce))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	paths := make([]string, 0, len(report.Packages))
	for _, pkg := range report.Packages {
		paths = append(paths, pkg.Path)
	}

	// greeting holds a function that never ran, so it is the less covered.
	if !slices.Equal(paths, []string{"greeting", "."}) {
		t.Fatalf("want packages [greeting .], least covered first; got %v", paths)
	}

	greeting, root := report.Packages[0], report.Packages[1]

	if names := functionNames(greeting); !slices.Equal(names, []string{"Greet", "(*Greeter).Farewell"}) {
		t.Errorf("want Greet and (*Greeter).Farewell unrun in greeting, got %v", names)
	}

	if names := functionNames(root); !slices.Equal(names, []string{"main"}) {
		t.Errorf("want main unrun in the root package, got %v", names)
	}

	assertSpan(t, greeting.Functions[0], "the empty-name branch", 11, 12, 1, false)
	assertSpan(t, greeting.Functions[1], "the whole of Farewell", 24, 28, 3, true)
	assertSpan(t, root.Functions[0], "the Farewell call", 18, 18, 1, false)

	if report.Covered() == 0 || report.Covered() >= report.Statements() {
		t.Errorf("want some but not all statements covered, got %d of %d",
			report.Covered(), report.Statements())
	}
}

func TestMeasureRefusesAFailedExercise(t *testing.T) {
	t.Parallel()

	_, err := enginecoverage.Measure(samplePlan(t, func(binPath string, env []string) error {
		_ = greetOnce(binPath, env)

		return errSuiteRed
	}))
	if !errors.Is(err, errSuiteRed) {
		t.Errorf("want the exercise's own failure, got %v", err)
	}
}

func TestMeasureRefusesAnExerciseThatNeverRanTheBinary(t *testing.T) {
	t.Parallel()

	_, err := enginecoverage.Measure(samplePlan(t, func(string, []string) error { return nil }))
	if !errors.Is(err, enginecoverage.ErrNoCoverageData) {
		t.Errorf("want ErrNoCoverageData, got %v", err)
	}
}

func functionNames(pkg enginecoverage.Package) []string {
	names := make([]string, 0, len(pkg.Functions))
	for _, fn := range pkg.Functions {
		names = append(names, fn.Name)
	}

	return names
}

// assertSpan wants function's unrun code as one span running from at or before
// through to last, merging the given number of blocks.
func assertSpan(t *testing.T, function enginecoverage.Function, what string, through, last, blocks int, whole bool) {
	t.Helper()

	if len(function.Uncovered) != 1 {
		t.Fatalf("%s: want one unrun span in %s, got %+v", what, function.Name, function.Uncovered)
	}

	span := function.Uncovered[0]
	if span.FirstLine > through || span.LastLine != last || span.Blocks != blocks {
		t.Errorf("%s: want lines ..%d-%d over %d block(s) in %s, got %+v",
			what, through, last, blocks, function.Name, span)
	}

	if ranNone := function.UncoveredStatements() == function.Statements; ranNone != whole {
		t.Errorf("%s: want never-ran=%v for %s, got %d of %d unrun",
			what, whole, function.Name, function.UncoveredStatements(), function.Statements)
	}
}
