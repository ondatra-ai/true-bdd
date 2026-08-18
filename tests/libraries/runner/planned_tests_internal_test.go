package runner

import (
	"strings"
	"testing"
)

// declared is the set of top-level tests a suite binary holds.
func declared() []string {
	return []string{"TestE2E001", "TestE2E011", "TestE2E023", "TestScenarioCoverage"}
}

// With no filter the plan is everything, which is what makes the
// report's denominator the run's intent rather than its progress.
func TestPlannedTestsWithoutFiltersPlansEverything(t *testing.T) {
	t.Parallel()

	planned, err := PlannedTests(declared(), "", "")
	if err != nil {
		t.Fatalf("PlannedTests: %v", err)
	}

	if len(planned) != len(declared()) {
		t.Fatalf("want all %d planned, got %v", len(declared()), planned)
	}
}

// -run is an unanchored regexp, exactly as `go test` treats it — so a
// pattern that names no prefix at all still selects, which is why
// `-run E2E` picks the scenarios and leaves the guards alone.
func TestPlannedTestsRunIsUnanchored(t *testing.T) {
	t.Parallel()

	planned, err := PlannedTests(declared(), "E2E", "")
	if err != nil {
		t.Fatalf("PlannedTests: %v", err)
	}

	want := "TestE2E001|TestE2E011|TestE2E023"
	if strings.Join(planned, "|") != want {
		t.Errorf("planned = %v, want %s", planned, want)
	}
}

// -skip removes from whatever -run selected, and the two compose the
// way they do on the command line.
func TestPlannedTestsSkipRemovesFromTheRunSet(t *testing.T) {
	t.Parallel()

	planned, err := PlannedTests(declared(), "TestE2E", "011$")
	if err != nil {
		t.Fatalf("PlannedTests: %v", err)
	}

	want := "TestE2E001|TestE2E023"
	if strings.Join(planned, "|") != want {
		t.Errorf("planned = %v, want %s", planned, want)
	}
}

// A pattern that will not compile is an error rather than a filter that
// silently matches nothing — the latter would report a plan of zero and
// a run of zero, which looks exactly like a suite with no tests.
func TestPlannedTestsRefusesABadPattern(t *testing.T) {
	t.Parallel()

	_, err := PlannedTests(declared(), "TestE2E(", "")
	if err == nil {
		t.Fatal("want a compile error, got nil")
	}
}
