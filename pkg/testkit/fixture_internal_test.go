package testkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

func TestPrepCmdsSeedsOneScriptAndRunsTheEditedListOnceBeforeSnapshot(t *testing.T) {
	t.Parallel()

	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "append", true: "replace"}[replace], func(t *testing.T) {
			t.Parallel()
			state := hookScenarioRun(t)
			prep := "set -e\nmarker=script\nprintf '%s\\n' \"$marker\" >> prep-order\n"
			writeFixtureTestFile(t, filepath.Join(state.harness.FixtureDir(snapshotFixtureName), "prep.sh"), prep, 0o600)

			err := state.PrepareTree(snapshotFixtureName)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(state.Fixture.PrepCmds, []string{prep}) {
				t.Fatalf("prep.sh must seed one command: %#v", state.Fixture.PrepCmds)
			}

			want := "script\nappended\nlast\n"

			if replace {
				state.Fixture.PrepCmds = []string{"printf 'replacement\\n' >> prep-order"}
				want = "replacement\nappended\nlast\n"
			}

			state.Fixture.PrepCmds = append(state.Fixture.PrepCmds,
				"printf 'appended\\n' >> prep-order", "printf 'last\\n' >> prep-order")
			binary := filepath.Join(t.TempDir(), "service")
			writeFixtureTestFile(t, binary, "#!/bin/sh\ncat prep-order\nprintf produced > output\n", 0o700)

			err = state.Run("exercise", RunOptions{Binary: binary})
			if err != nil {
				t.Fatal(err)
			}

			if state.Result.ExitCode != 0 || state.Result.Stdout != want {
				t.Fatalf("preparation did not run once in order: %+v", state.Result)
			}

			if len(state.Result.Diff) != 1 || state.Result.Diff[0].Path != "output" {
				t.Fatalf("preparation entered the service diff: %+v", state.Result.Diff)
			}
		})
	}
}

func TestExpectedExitCodeSuppliesTheReportExpectationWithoutCheckingTheRun(t *testing.T) {
	t.Parallel()
	fixtures, session := t.TempDir(), t.TempDir()

	input := filepath.Join(fixtures, snapshotFixtureName, "input")

	err := os.MkdirAll(input, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "service")
	writeFixtureTestFile(t, binary, "#!/bin/sh\nexit 7\n", 0o700)
	shared := &harness{BinPath: binary, SessionRoot: session, FixturesDir: fixtures,
		FixtureStep: `fixture (.+)`, Modes: runner.Modes{Services: ProxyModeLive, Tests: ProxyModeReplay}}

	t.Cleanup(func() {
		data, err := Read(filepath.Join(session, snapshotFixtureName, SpawnLogDir, ManifestSnapshotFile))
		if err != nil {
			t.Fatal(err)
		}

		var report ManifestSnapshot

		err = json.Unmarshal(data, &report)
		if err != nil {
			t.Fatal(err)
		}

		if report.ExpectedExit != 42 {
			t.Fatalf("report expectation = %d, want the Then-supplied 42", report.ExpectedExit)
		}
	})

	t.Run("run", func(t *testing.T) {
		t.Parallel()

		state, err := newScenarioRun(shared)(&bddgo.World{T: t, Scenario: Scenario{ID: snapshotFixtureName,
			Steps: []bddgo.Step{{Text: "fixture sample"}}}})
		if err != nil {
			t.Fatal(err)
		}

		err = state.PrepareTree(snapshotFixtureName)
		if err != nil {
			t.Fatal(err)
		}

		state.Fixture.ExpectedExitCode = 23

		err = state.Run("exercise", RunOptions{})
		if err != nil {
			t.Fatal(err)
		}

		if state.Result.ExitCode != 7 {
			t.Fatalf("observed status = %d, want 7", state.Result.ExitCode)
		}

		state.Fixture.ExpectedExitCode = 42

		err = CheckExitCode(state.Result.ExitCode, 7)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func writeFixtureTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTimeoutReportsTheEffectiveSuiteOrScenarioTimeout(t *testing.T) {
	for _, scenarioTimeout := range []time.Duration{0, 3 * time.Second} {
		t.Run(scenarioTimeout.String(), func(t *testing.T) {
			shared := fixtureTestHarness(t)
			shared.Timeout = 2 * time.Second

			var state *ScenarioRun

			if !t.Run("run", func(t *testing.T) {
				state = newFixtureTestRun(t, shared)
				state.Scenario.Timeout = scenarioTimeout
				state.Fixture.Timeout = time.Nanosecond

				err := state.Run("exercise", RunOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if state.Result.ExitCode != 0 || state.Result.Stdout != "observed" {
					t.Fatalf("fixture timeout prevented the invocation: %+v", state.Result)
				}
			}) {
				t.Fatal("Scenario run failed")
			}

			want := 2 * time.Second
			if scenarioTimeout > 0 {
				want = 3 * time.Second
			}

			var report ManifestSnapshot

			readFixtureTestReport(t, state, ManifestSnapshotFile, &report)

			if state.Fixture.Timeout != want || report.TimeoutMs != want.Milliseconds() {
				t.Fatalf("effective timeout = %s, report = %d ms, want %s",
					state.Fixture.Timeout, report.TimeoutMs, want)
			}
		})
	}
}

func TestStdoutRegexesSupplyTheReportWithoutCheckingTheRun(t *testing.T) {
	shared := fixtureTestHarness(t)

	var state *ScenarioRun

	if !t.Run("run", func(t *testing.T) {
		state = newFixtureTestRun(t, shared)
		state.Fixture.StdoutRegexes = []*regexp.Regexp{regexp.MustCompile(`^missing$`)}

		err := state.Run("exercise", RunOptions{})
		if err != nil {
			t.Fatal(err)
		}

		err = CheckStdout(state.Result.Stdout, regexp.MustCompile(`^observed$`))
		if err != nil {
			t.Fatal(err)
		}

		err = CheckStdout(state.Result.Stdout, state.Fixture.StdoutRegexes[0])
		if err == nil {
			t.Fatal("explicit missing-pattern assertion passed")
		}
	}) {
		t.Fatal("report-only patterns failed the Scenario run")
	}

	var report ManifestSnapshot

	readFixtureTestReport(t, state, ManifestSnapshotFile, &report)

	if !reflect.DeepEqual(report.StdoutRegexes, []string{`^missing$`}) {
		t.Fatalf("reported stdout patterns = %#v, want the unmatched pattern", report.StdoutRegexes)
	}

	var record HarnessRecord

	readFixtureTestReport(t, state, HarnessRecordFile, &record)

	if record.Verdict != VerdictPass || len(record.Failures) != 0 {
		t.Fatalf("report-only patterns changed the run's verdict: %+v", record)
	}
}

func fixtureTestHarness(t *testing.T) *harness {
	t.Helper()
	fixtures := t.TempDir()

	err := os.MkdirAll(filepath.Join(fixtures, snapshotFixtureName, "input"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "service")
	writeFixtureTestFile(t, binary, "#!/bin/sh\nprintf observed\n", 0o700)

	return &harness{BinPath: binary, SessionRoot: t.TempDir(), FixturesDir: fixtures,
		FixtureStep: `report fixture (.+)`, Modes: runner.Modes{Services: ProxyModeLive, Tests: ProxyModeReplay}}
}

func newFixtureTestRun(t *testing.T, shared *harness) *ScenarioRun {
	t.Helper()

	state, err := newScenarioRun(shared)(&bddgo.World{T: t, Scenario: Scenario{ID: snapshotFixtureName,
		Steps: []bddgo.Step{{Text: "report fixture sample"}}}})
	if err != nil {
		t.Fatal(err)
	}

	err = state.PrepareTree(snapshotFixtureName)
	if err != nil {
		t.Fatal(err)
	}

	return state
}

func readFixtureTestReport(t *testing.T, state *ScenarioRun, name string, report any) {
	t.Helper()

	data, err := Read(filepath.Join(state.harness.SessionRoot, snapshotFixtureName, SpawnLogDir, name))
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, report)
	if err != nil {
		t.Fatal(err)
	}
}
