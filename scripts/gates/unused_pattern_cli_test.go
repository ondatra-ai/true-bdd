package gates_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestUnusedPatternCLIControlsOnlyFirstProposalBehindHarnessShim(t *testing.T) {
	utility := recordingTestUtility(t)

	shims, otherShim, programs := t.TempDir(), t.TempDir(), t.TempDir()
	for _, directory := range []string{shims, otherShim} {
		err := utility.Substitute(directory, []proxycli.Dependency{{Name: "claude"}})
		if err != nil {
			t.Fatal(err)
		}
	}

	program := filepath.Join(programs, "claude")
	addRecordingGo(t, programs)
	writeRecordingProbe(t, program, "#!/bin/sh\nprintf '<%s>\\n' \"$@\" >&2\ncat\nexit 7\n")

	err := os.Chmod(program, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	fixture, err := filepath.Abs("../../tests/true-bdd/fixtures/concept-steps-unused/cli")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", otherShim+":"+programs+":/usr/bin:/bin")

	for _, transport := range []string{"argv", "stdin"} {
		t.Run(transport, func(t *testing.T) {
			root := t.TempDir()
			run := &testkit.ScenarioRun{}
			run.SetEnv(proxycli.Environment("observe", shims, t.TempDir(), t.TempDir(), []string{shims, otherShim})...)
			// Restore the second shim to ensure the fixture skips every proxy, including another caller's.
			run.SetEnv("PATH=" + strings.Join([]string{shims, otherShim, programs, "/usr/bin", "/bin"}, ":"))
			proxycli.FallbackPath(run, fixture)
			assertUnusedPatternForwarded(t, shims, root, run.Environment(), []string{"--other", "two words", ""}, "before\n\n")

			proposal := "Propose the step definition patterns of one concept for one suite,\nas data."

			args, input := []string{"--system-prompt", proposal}, "payload\n\n"
			if transport == "stdin" {
				args, input = []string{"-p"}, proposal+"\n\n"
			}

			result := runUnusedPatternCLI(t, shims, root, run.Environment(), args, input)
			if result.Code != 0 {
				t.Fatalf("planted proposal: %+v", result)
			}

			assertUnusedPatternProposal(t, result.Stdout)
			assertUnusedPatternForwarded(t, shims, root, run.Environment(), args, input)
			assertUnusedPatternForwarded(t, shims, root, run.Environment(), []string{"--verification"}, "after\n\n")
		})
	}
}

func runUnusedPatternCLI(t *testing.T, shims, root string, environment, args []string, input string) cli.Result {
	t.Helper()

	result, err := spec.Run(append([]string{filepath.Join(shims, "claude")}, args...), cli.Options{
		Dir: root, Env: cli.Inherit().Set(environment...), Stdin: strings.NewReader(input), Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func assertUnusedPatternForwarded(t *testing.T, shims, root string, environment, args []string, input string) {
	t.Helper()
	result := runUnusedPatternCLI(t, shims, root, environment, args, input)

	wantErrors := "<" + strings.Join(args, ">\n<") + ">\n"
	if result.Code != 7 || result.Stdout != input || result.Stderr != wantErrors {
		t.Fatalf("forwarding changed argv, stdin, streams or status: %+v", result)
	}
}

func assertUnusedPatternProposal(t *testing.T, output string) {
	t.Helper()

	var response struct {
		StructuredOutput struct {
			Patterns []struct {
				Pattern  string `json:"pattern"`
				Captures []any  `json:"captures"`
			} `json:"patterns"`
		} `json:"structured_output"`
	}

	err := json.Unmarshal([]byte(output), &response)
	if err != nil {
		t.Fatal(err)
	}

	patterns := response.StructuredOutput.Patterns
	if len(patterns) != 2 {
		t.Fatalf("expected a covering pattern plus one unused pattern: %s", output)
	}

	for index, pattern := range patterns {
		compiled, err := regexp.Compile(pattern.Pattern)
		if err != nil || compiled.NumSubexp() != len(pattern.Captures) {
			t.Fatalf("invalid planted pattern: %+v, %v", pattern, err)
		}

		for _, step := range []string{"the Owner halves\u00a06", "the Owner halves\u00a0-7"} {
			if compiled.MatchString(step) != (index == 0) {
				t.Errorf("pattern %q: unexpected binding for %q", pattern.Pattern, step)
			}
		}
	}
}
