package testrunner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

const goTestScriptCommand = "sh runner.sh -json"

func TestSetRerunRequiresEachMembersOwnPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	script := `#!/bin/sh
set -e
[ "$#" = 3 ]
[ "$2" = '-run' ]
[ "$3" = '^TestA$|^TestB$|^TestMissing$' ]
printf '%s\n' '{"Action":"pass","Package":"host/tests","Test":"TestA"}'\
 '{"Action":"fail","Package":"host/tests","Test":"TestB"}'\
 '{"Action":"pass","Package":"other/tests","Test":"TestMissing"}'
`

	err := disk.Write(filepath.Join(root, "runner.sh"), []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testrunner.Config{Command: goTestScriptCommand, ConfigFile: filepath.Join(root, "go.mod")}

	members := make([]*testrunner.FailingTest, 0, 3)
	for _, name := range []string{"TestA", "TestB", "TestMissing"} {
		members = append(members, &testrunner.FailingTest{TestName: "host/tests::" + name, RunnerConfig: cfg})
	}

	err = testrunner.NewGoTestRunner(nil).RunSet(context.Background(), members)
	if err != nil {
		t.Fatal(err)
	}

	if !members[0].LastRunPassed || members[1].LastRunPassed || members[2].LastRunPassed {
		t.Fatalf("incorrect member verdicts: %+v %+v %+v", members[0], members[1], members[2])
	}
}

type setExample struct {
	name                      string
	runner                    testrunner.Runner
	members                   []string
	command, expected, report string
	passed                    []bool
}

func setExamples() []setExample {
	return []setExample{
		{
			"mixed Go build",
			testrunner.NewGoTestRunner(nil),
			[]string{"host/broken::<build>", "host/tests::TestB"},
			goTestScriptCommand,
			goMachineFlag,
			`{"Action":"pass","Package":"host/broken"}
{"Action":"pass","Package":"host/tests","Test":"TestB"}`,
			[]bool{true, true},
		},
		{
			"Jest alternation and absent result",
			testrunner.NewJestRunner(nil),
			[]string{"test.js::first (one)", "test.js::second", "test.js::absent"},
			"sh runner.sh --json",
			`--json --testNamePattern ^first \(one\)$|^second$|^absent$`,
			`{"testResults":[
{"name":"test.js","assertionResults":[
{"fullName":"first (one)","status":"passed"},
{"fullName":"second","status":"pending"}]}]}`,
			[]bool{true, false, false},
		},
		{
			"Playwright alternation",
			testrunner.NewPlaywrightRunner(nil),
			[]string{"spec.ts::spec.ts > first", "spec.ts::spec.ts > second"},
			"sh runner.sh --reporter=json",
			`--reporter=json --grep spec\.ts first|spec\.ts second`,
			`{"suites":[
{"title":"spec.ts","file":"spec.ts","specs":[
{"title":"first","tests":[{"results":[{"status":"passed"}]}]},
{"title":"second","tests":[{"results":[{"status":"skipped"}]}]}]}]}`,
			[]bool{true, false},
		},
		{
			"Playwright startup",
			testrunner.NewPlaywrightRunner(nil),
			[]string{"<startup>", "spec.ts::spec.ts > first"},
			"sh runner.sh --reporter=json",
			playwrightMachineFlag,
			`{"stats":{"expected":1},"suites":[
{"title":"spec.ts","file":"spec.ts","specs":[
{"title":"first","tests":[{"results":[{"status":"passed"}]}]}]}]}`,
			[]bool{true, true},
		},
	}
}

func TestSetRerunNativeSelectionsAndSyntheticMembers(t *testing.T) {
	t.Parallel()

	for _, example := range setExamples() {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			script := "#!/bin/sh\nset -e\n[ \"$*\" = '" + example.expected + "' ]\nprintf '%s\\n' '" + example.report + "'\n"

			err := disk.Write(filepath.Join(root, "runner.sh"), []byte(script), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			cfg := testrunner.Config{Command: example.command, ConfigFile: filepath.Join(root, "config")}

			members := make([]*testrunner.FailingTest, 0, len(example.members))
			for _, name := range example.members {
				members = append(members, &testrunner.FailingTest{TestName: name, RunnerConfig: cfg})
			}

			err = example.runner.RunSet(context.Background(), members)
			if err != nil {
				t.Fatal(err)
			}

			for index, member := range members {
				if member.LastRunPassed != example.passed[index] {
					t.Fatalf("%s passed=%v; want %v, output=%s",
						member.TestName, member.LastRunPassed, example.passed[index], member.FailureOutput,
					)
				}

				if !example.passed[index] && !strings.Contains(member.FailureOutput, "did not run") {
					t.Fatalf("missing execution finding: %s", member.FailureOutput)
				}
			}
		})
	}
}
