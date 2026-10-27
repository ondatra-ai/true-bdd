package testrunner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

const loadName = "tests/unit/greeting.test.js::<load>"
const loadReport = `{"testResults":[{"name":"tests/unit/greeting.test.js",
"status":"failed","message":"Cannot find greeting","assertionResults":[]}]}`
const loadedReport = `{"testResults":[{"name":"tests/unit/greeting.test.js","status":"passed",
"assertionResults":[{"fullName":"greets","status":"passed"}]}]}`

func jestCommand(t *testing.T, report, expected string) testrunner.Config {
	t.Helper()
	root := t.TempDir()

	script := "#!/bin/sh\nset -e\n[ \"$*\" = '" + expected + "' ]\nprintf '%s\\n' '" + report + "'\n"
	if strings.Contains(report, `"status":"failed"`) {
		script += "exit 1\n"
	}

	err := disk.Write(filepath.Join(root, "runner.sh"), []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return testrunner.Config{Command: "sh runner.sh --json", ConfigFile: filepath.Join(root, "config")}
}

func TestJestDiscoversFileThatFailsToLoad(t *testing.T) {
	t.Parallel()
	cfg := jestCommand(t, loadReport, "--json")

	failures, err := testrunner.NewJestRunner(nil).Discover(context.Background(), cfg, "greeter", "jest")
	if err != nil {
		t.Fatal(err)
	}

	if len(failures) != 1 {
		t.Fatalf("failures=%v; want one load member", failures)
	}

	member := failures[0]
	if member.TestName != loadName || member.FilePath != "tests/unit/greeting.test.js" ||
		member.FailureOutput != "Cannot find greeting" ||
		member.ID != testrunner.BuildID("greeter", "jest", testrunner.FrameworkJest, loadName) {
		t.Fatalf("wrong member: %+v", member)
	}
}

func TestJestLoadMemberRequiresItsOwnSuccessfulFile(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, report, finding string
		passed                bool
	}{
		{"loads and passes", loadedReport, "", true},
		{"still fails to load", loadReport, "Cannot find greeting", false},
		{"missing file", `{"testResults":[{"name":"other.js","status":"passed",
"assertionResults":[{"fullName":"greets","status":"passed"}]}]}`, "did not run", false},
		{"loaded but test fails", `{"testResults":[{"name":"tests/unit/greeting.test.js","status":"failed",
"assertionResults":[{"fullName":"greets","status":"failed"}]}]}`, "", false},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			for _, single := range []bool{true, false} {
				passed, output := jestLoadVerdict(t, example.report, single)
				if passed != example.passed {
					t.Fatalf("single=%v passed=%v want %v; output=%s", single, passed, example.passed, output)
				}

				if !strings.Contains(output, example.finding) {
					t.Fatalf("missing finding %q: %s", example.finding, output)
				}
			}
		})
	}
}

func jestLoadVerdict(t *testing.T, report string, single bool) (bool, string) {
	t.Helper()

	absolute, err := filepath.Abs("tests/unit/greeting.test.js")
	if err != nil {
		t.Fatal(err)
	}

	cfg := jestCommand(t, report, "--json --runTestsByPath "+absolute)
	member := &testrunner.FailingTest{TestName: loadName, RunnerConfig: cfg}
	runner := testrunner.NewJestRunner(nil)

	var (
		passed bool
		output string
	)
	if single {
		passed, output, err = runner.RunOne(context.Background(), member)
	} else {
		err = runner.RunSet(context.Background(), []*testrunner.FailingTest{member})
		passed = member.LastRunPassed
		output = member.FailureOutput
	}

	if err != nil {
		t.Fatal(err)
	}

	return passed, output
}

func TestJestDiscoveryRequiresFileFailureEvidence(t *testing.T) {
	t.Parallel()

	for _, report := range []string{loadedReport, emptyJestReport} {
		cfg := jestCommand(t, report, "--json")

		failures, err := testrunner.NewJestRunner(nil).Discover(context.Background(), cfg, "", "")
		if err != nil || len(failures) != 0 {
			t.Fatalf("passing or absent file became a failure: %v, %v", failures, err)
		}
	}
}

func TestJestMixedLoadSetSelectsFilesAndKeepsOwnVerdicts(t *testing.T) {
	t.Parallel()

	absolute, err := filepath.Abs("tests/unit/greeting.test.js")
	if err != nil {
		t.Fatal(err)
	}

	cfg := jestCommand(t, loadedReport, "--json --runTestsByPath "+absolute)
	members := []*testrunner.FailingTest{
		{TestName: loadName, RunnerConfig: cfg},
		{TestName: "tests/unit/greeting.test.js::greets", RunnerConfig: cfg},
		{TestName: "tests/unit/greeting.test.js::absent", RunnerConfig: cfg},
	}

	err = testrunner.NewJestRunner(nil).RunSet(context.Background(), members)
	if err != nil {
		t.Fatal(err)
	}

	if !members[0].LastRunPassed || !members[1].LastRunPassed || members[2].LastRunPassed {
		t.Fatalf("wrong mixed-set verdicts: %+v", members)
	}
}

func TestJestLoadMemberAggregatesTheSameFileAcrossProjects(t *testing.T) {
	t.Parallel()

	report := `{"testResults":[
 {"name":"tests/unit/greeting.test.js","status":"failed","message":"Cannot find greeting","assertionResults":[]},
 {"name":"tests/unit/greeting.test.js","status":"failed","message":"Cannot find greeting","assertionResults":[]},
 {"name":"tests/unit/greeting.test.js","status":"passed",
 "assertionResults":[{"fullName":"greets","status":"passed"}]}]}`
	cfg := jestCommand(t, report, "--json")

	failures, err := testrunner.NewJestRunner(nil).Discover(context.Background(), cfg, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(failures) != 1 {
		t.Fatalf("one file yielded %d load members", len(failures))
	}

	for _, single := range []bool{true, false} {
		passed, output := jestLoadVerdict(t, report, single)
		if passed || !strings.Contains(output, "Cannot find greeting") {
			t.Fatalf("a failing file passed: %v %s", passed, output)
		}
	}
}
