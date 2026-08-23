package testrunner

import (
	"errors"
	"reflect"
	"testing"
)

// goArgv spells the `go test ...` prefix every Go command shares, so a
// case below shows only the part that distinguishes it.
func goArgv(rest ...string) []string {
	return append([]string{"go", "test"}, rest...)
}

func TestSplitCommandKeepsQuotedArgumentsWhole(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		command string
		want    []string
	}{
		"plain": {
			command: "go test -json ./...",
			want:    goArgv(goTestJSONFlag, "./..."),
		},
		// The reason this splitter exists at all: strings.Fields turns
		// the filter into two arguments and quietly changes which tests
		// run, with no error to show for it.
		"single-quoted regex": {
			command: "go test -json -run '^TestGreen$' ./tests/...",
			want:    goArgv(goTestJSONFlag, goTestRunFlag, "^TestGreen$", "./tests/..."),
		},
		"double-quoted with spaces": {
			command: `npx playwright test --grep "home page greets"`,
			want:    []string{"npx", "playwright", "test", "--grep", "home page greets"},
		},
		"quote inside quote": {
			command: `sh -c "echo 'hi there'"`,
			want:    []string{"sh", "-c", "echo 'hi there'"},
		},
		"empty quoted argument survives": {
			command: `npx jest --json --testPathPattern ''`,
			want:    []string{"npx", "jest", "--json", "--testPathPattern", ""},
		},
		"runs of whitespace collapse": {
			command: "go   test\t-json",
			want:    goArgv(goTestJSONFlag),
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := SplitCommand(testCase.command)
			if err != nil {
				t.Fatalf("SplitCommand(%q): %v", testCase.command, err)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("SplitCommand(%q) = %q, want %q", testCase.command, got, testCase.want)
			}
		})
	}
}

// Half-quoted is refused rather than guessed: silently splitting
// `-run '^Test` into two arguments would run a different set of tests
// than the spec asked for.
func TestSplitCommandRejectsUnterminatedQuote(t *testing.T) {
	t.Parallel()

	_, err := SplitCommand("go test -run '^TestGreen")
	if !errors.Is(err, ErrUnterminatedQuote) {
		t.Fatalf("want ErrUnterminatedQuote, got %v", err)
	}
}

// `”` is the same mistake as an empty string: it closes an argument
// with no content, and a token count alone would call that a command and
// hand exec an empty binary name.
func TestSplitCommandRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"", "   ", "\t\n", "''", `""`, "'' -json ./..."} {
		_, err := SplitCommand(command)
		if !errors.Is(err, ErrEmptyCommand) {
			t.Errorf("SplitCommand(%q): want ErrEmptyCommand, got %v", command, err)
		}
	}
}

// The runners parse machine-readable output; a command missing the flag
// that produces it yields a report with no failures, so the walk converges
// on zero items and reports a false green — refused at startup.
func TestValidateCommandRequiresMachineReadableOutput(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		framework string
		command   string
		wantErr   bool
	}{
		"go-test with -json":            {FrameworkGoTest, "go test -json ./...", false},
		"go-test without -json":         {FrameworkGoTest, "go test ./...", true},
		"jest with --json":              {FrameworkJest, "npx jest --json", false},
		"playwright with reporter":      {FrameworkPlaywright, "npx playwright test --reporter=json", false},
		"playwright with html reporter": {FrameworkPlaywright, "npx playwright test --reporter=html", true},
		"playwright without reporter":   {FrameworkPlaywright, "npx playwright test", true},
		// Every spelling below is output the runners actually parse: go test
		// accepts either dash count, a composite reporter emits JSON in
		// either position, and a value-taking flag may be space-separated.
		"go-test with --json":               {FrameworkGoTest, "go test --json ./...", false},
		"playwright with json,line":         {FrameworkPlaywright, "npx playwright test --reporter=json,line", false},
		"playwright with line,json":         {FrameworkPlaywright, "npx playwright test --reporter=line,json", false},
		"playwright with spaced value":      {FrameworkPlaywright, "npx playwright test --reporter json", false},
		"playwright reporter with no value": {FrameworkPlaywright, "npx playwright test --reporter", true},
		// An unknown framework is the dispatcher's error to report, not
		// a second error here for the same mistake.
		"unknown framework": {"rspec", "bundle exec rspec", false},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCommand(testCase.framework, testCase.command)
			if testCase.wantErr && !errors.Is(err, ErrCommandNotMachineReadable) {
				t.Fatalf("want ErrCommandNotMachineReadable, got %v", err)
			}

			if !testCase.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateCommandRejectsUnsplittableCommand(t *testing.T) {
	t.Parallel()

	err := ValidateCommand(FrameworkGoTest, "go test -json -run '^Test")
	if !errors.Is(err, ErrUnterminatedQuote) {
		t.Fatalf("want ErrUnterminatedQuote, got %v", err)
	}
}

// The working directory is what lets `npx` find a suite's own install,
// and what a go-test layer leaves to the engine's own cwd.
func TestCommandDir(t *testing.T) {
	t.Parallel()

	got := CommandDir(Config{ConfigFile: "tests/playwright.config.ts"})
	if got != "tests" {
		t.Errorf("with a config file: got %q, want %q", got, "tests")
	}

	if CommandDir(Config{}) != "" {
		t.Errorf("without a config file the engine's own cwd is inherited, got %q", CommandDir(Config{}))
	}
}

// A rerun must inherit the build tags and test-binary flags that made
// the test discoverable: reassembling the invocation from scratch would
// compile a different set of packages and never reproduce the failure.
func TestAppendGoRunFilterNarrowsTheLayersOwnCommand(t *testing.T) {
	t.Parallel()

	base := goArgv("-tags", "bdd", goTestJSONFlag, "./tests/bdd-cli/", "-mode=replay")

	got := appendGoRunFilter(base, "TestBDDFixtures/help-flag")
	want := append(append([]string{}, base...), goTestRunFlag, "^TestBDDFixtures$/^help-flag$")

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// The filter is appended, never spliced in front of the package
	// selector the spec placed there.
	if got[len(base)] != goTestRunFlag {
		t.Errorf("filter must be appended, got %q", got)
	}

	// A package that failed to compile has no test to name, so the
	// command runs unfiltered and the compile is the verdict.
	if unfiltered := appendGoRunFilter(base, goBuildFailureMarker); !reflect.DeepEqual(unfiltered, base) {
		t.Errorf("build-failure marker must not be filtered: got %q", unfiltered)
	}
}

func TestCommandArgvAttributesAMalformedRerunCommand(t *testing.T) {
	t.Parallel()

	_, err := commandArgv(&FailingTest{
		TestName:     "pkg::TestFoo",
		RunnerConfig: Config{Command: "go test -json -run '^Test"},
	})
	if !errors.Is(err, ErrUnterminatedQuote) {
		t.Fatalf("want ErrUnterminatedQuote, got %v", err)
	}
}
