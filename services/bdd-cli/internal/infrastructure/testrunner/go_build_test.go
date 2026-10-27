package testrunner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestGoBuildMemberRequiresItsOwnSuccessfulPackageResult(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, report string
		passed       bool
	}{
		{"pass", `{"Action":"pass","Package":"host/helper"}`, true},
		{"no test files", `{"Action":"output","Package":"host/helper","Output":"?   host/helper [no test files]\n"}
{"Action":"skip","Package":"host/helper"}`, true},
		{"build failure", `{"Action":"fail","Package":"host/helper","FailedBuild":"host/helper"}`, false},
		{"missing package", `{"Action":"pass","Package":"host/other"}`, false},
		{"test skip only", `{"Action":"skip","Package":"host/helper","Test":"TestSkipped"}`, false},
		{"failure overrides skip", `{"Action":"fail","Package":"host/helper"}
{"Action":"skip","Package":"host/helper"}`, false},
		{"test failure overrides pass", `{"Action":"fail","Package":"host/helper","Test":"TestBroken"}
{"Action":"pass","Package":"host/helper"}`, false},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()
			assertGoBuildVerdicts(t, example.report, example.passed)
		})
	}
}

func assertGoBuildVerdicts(t *testing.T, report string, wantPassed bool) {
	t.Helper()

	for _, judgment := range []string{"RunOne", "RunSet"} {
		t.Run(judgment, func(t *testing.T) {
			t.Parallel()
			member := goBuildMember(t, report)
			runner := testrunner.NewGoTestRunner(nil)

			var (
				passed bool
				err    error
			)

			if judgment == "RunOne" {
				passed, _, err = runner.RunOne(context.Background(), member)
			} else {
				err = runner.RunSet(context.Background(), []*testrunner.FailingTest{member})
				passed = member.LastRunPassed
			}

			if err != nil {
				t.Fatal(err)
			}

			if passed != wantPassed {
				t.Fatalf("build member passed=%v, want %v; report=%s", passed, wantPassed, report)
			}
		})
	}
}

func goBuildMember(t *testing.T, report string) *testrunner.FailingTest {
	t.Helper()
	root := t.TempDir()

	script := "#!/bin/sh\nset -e\n[ \"$*\" = '-json' ]\nprintf '%s\\n' '" + report + "'\n"

	err := disk.Write(filepath.Join(root, "runner.sh"), []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return &testrunner.FailingTest{
		TestName: "host/helper::<build>",
		RunnerConfig: testrunner.Config{
			Command: goTestScriptCommand, ConfigFile: filepath.Join(root, "go.mod"),
		},
	}
}
