package testrunner_test

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

const emptyJestReport = `{"testResults":[]}`
const goMachineFlag = "-json"
const playwrightMachineFlag = "--reporter=json"

func TestFailedCommandWithNoMemberStopsDiscovery(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, flag, report string
		runner             testrunner.Runner
	}{
		{testrunner.FrameworkJest, "--json", emptyJestReport, testrunner.NewJestRunner(nil)},
		{testrunner.FrameworkGoTest, goMachineFlag, `{"Action":"output","Output":"runner crashed"}`,
			testrunner.NewGoTestRunner(nil)},
		{testrunner.FrameworkPlaywright, playwrightMachineFlag, "",
			testrunner.NewPlaywrightRunner(nil)},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			for _, code := range []int{0, 3} {
				t.Run(strconv.Itoa(code), func(t *testing.T) {
					t.Parallel()
					cfg := failedCommand(t, example.flag, example.report, code)

					failures, err := example.runner.Discover(context.Background(), cfg, "", "")
					if code == 0 {
						if err != nil || len(failures) != 0 {
							t.Fatalf("green command failed: %v %v", err, failures)
						}

						return
					}

					assertCommandFailure(t, err, cfg.Command)
				})
			}
		})
	}
}

func failedCommand(t *testing.T, flag, report string, code int) testrunner.Config {
	t.Helper()
	root := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' '" + report + "'\necho output-tail-sentinel >&2\nexit " + strconv.Itoa(code) + "\n"

	err := disk.Write(filepath.Join(root, "runner.sh"), []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return testrunner.Config{Command: "sh runner.sh " + flag, ConfigFile: filepath.Join(root, "config")}
}

func assertCommandFailure(t *testing.T, err error, command string) {
	t.Helper()

	if err == nil {
		t.Fatal("failed command read as green")
	}

	for _, want := range []string{command, "code 3", "output-tail-sentinel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestFailedCommandWithNoMemberStopsReruns(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, flag, report, member string
		runner                     testrunner.Runner
	}{
		{testrunner.FrameworkJest, "--json", emptyJestReport, "file.js::works", testrunner.NewJestRunner(nil)},
		{testrunner.FrameworkGoTest, goMachineFlag, `{"Action":"output"}`, "host/tests::TestWorks",
			testrunner.NewGoTestRunner(nil)},
		{testrunner.FrameworkPlaywright, playwrightMachineFlag, `{"suites":[]}`, "spec.ts::works",
			testrunner.NewPlaywrightRunner(nil)},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()
			cfg := failedCommand(t, example.flag, example.report, 3)
			member := &testrunner.FailingTest{TestName: example.member, RunnerConfig: cfg}
			_, _, err := example.runner.RunOne(context.Background(), member)
			assertCommandFailure(t, err, cfg.Command)
			err = example.runner.RunSet(context.Background(), []*testrunner.FailingTest{member})
			assertCommandFailure(t, err, cfg.Command)
		})
	}
}

func TestPlaywrightStartupRemainsANamedMember(t *testing.T) {
	t.Parallel()
	cfg := failedCommand(t, playwrightMachineFlag, `{"suites":[],"errors":[{"message":"server failed"}]}`, 3)

	failures, err := testrunner.NewPlaywrightRunner(nil).Discover(context.Background(), cfg, "web", "browser")
	if err != nil {
		t.Fatal(err)
	}

	if len(failures) != 1 || failures[0].TestName != "<startup>" {
		t.Fatalf("lost startup member: %+v", failures)
	}
}
