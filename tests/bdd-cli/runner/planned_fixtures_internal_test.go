package runner

import (
	"strings"
	"testing"
)

// The suite's own shape: a parent test with one subtest per fixture,
// and three real fixture names to filter over — real ones because the
// overlapping prefixes (us-create-…, us-refine-…) are exactly what the
// unanchored matching has to get right.
const (
	testRoot      = "TestBDDFixtures"
	helpFixture   = "help-flag"
	createFixture = "us-create-happy-path"
	refineFixture = "us-refine-fix-steps"
)

func TestPlannedFixtures(t *testing.T) {
	t.Parallel()

	names := []string{helpFixture, createFixture, "us-create-fix-happy-path", refineFixture}

	cases := []struct {
		name string
		run  string
		skip string
		want []string
	}{
		{
			name: "no filters plans every fixture",
			want: names,
		},
		{
			name: "parent-only pattern plans every fixture",
			run:  testRoot,
			want: names,
		},
		{
			name: "a fixture pattern plans just that fixture",
			run:  testRoot + "/" + refineFixture,
			want: []string{refineFixture},
		},
		{
			// The property that makes the whole reimplementation worth
			// pinning: `go test` matches unanchored, so a pattern naming
			// one fixture also plans every fixture whose name contains it.
			name: "matching is unanchored",
			run:  testRoot + "/" + createFixture,
			want: []string{createFixture},
		},
		{
			name: "an alternation plans each alternative",
			run:  testRoot + "/(" + helpFixture + "|" + refineFixture + ")",
			want: []string{helpFixture, refineFixture},
		},
		{
			name: "a pattern that does not name this parent plans nothing",
			run:  "TestSomethingElse/" + refineFixture,
			want: []string{},
		},
		{
			name: "skip removes from the plan",
			skip: testRoot + "/us-create",
			want: []string{helpFixture, refineFixture},
		},
		{
			name: "run and skip compose",
			run:  testRoot + "/us-",
			skip: testRoot + "/fix",
			want: []string{createFixture},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := PlannedFixtures(testRoot, names, testCase.run, testCase.skip)
			if err != nil {
				t.Fatalf("PlannedFixtures: %v", err)
			}

			if strings.Join(got, ",") != strings.Join(testCase.want, ",") {
				t.Fatalf("planned = %v, want %v", got, testCase.want)
			}
		})
	}
}

// A pattern `go test` itself would refuse must not be silently read as
// "everything" — that would report a denominator larger than the run.
func TestPlannedFixturesRejectsBadPattern(t *testing.T) {
	t.Parallel()

	_, err := PlannedFixtures(testRoot, []string{"a"}, testRoot+"/(unclosed", "")
	if err == nil {
		t.Fatal("expected an error for an uncompilable -run pattern")
	}
}
