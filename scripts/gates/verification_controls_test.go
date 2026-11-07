package gates_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

const (
	verificationCodex            = "codex"
	verificationSystemPromptFlag = "--system-prompt"
)

type fixtureVerificationControl struct {
	ID         string `json:"id"`
	Block      string `json:"block"`
	Concept    string `json:"concept"`
	Source     string `json:"source"`
	Expected   string `json:"expected_verdict"`
	Category   string `json:"category"`
	Definition struct {
		File     string `json:"file"`
		Function string `json:"function"`
		Pattern  string `json:"pattern"`
	} `json:"definition"`
}

type archivedVerificationControl struct {
	Item    string `json:"item"`
	Context string `json:"context_yaml"`
	Sources string `json:"whole_sources"`
	System  string `json:"recorded_system"`
}

func verificationFixtureControls(t *testing.T, fixture string) []fixtureVerificationControl {
	t.Helper()

	data, err := disk.Read(filepath.Join(fixture, "controls.json"))
	if err != nil {
		t.Fatal(err)
	}

	var controls []fixtureVerificationControl

	err = json.Unmarshal(data, &controls)
	if err != nil {
		t.Fatal(err)
	}

	return controls
}

func verificationFixturePaths(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob("../../tests/true-bdd/fixtures/verification-controls-*")
	if err != nil || len(paths) != 10 {
		t.Fatalf("control fixtures: %v, %v", paths, err)
	}

	return paths
}

func TestVerificationControlsPreserveCompleteInputs(t *testing.T) {
	t.Parallel()

	expected := archivedVerificationHashes()
	seen := make(map[string]bool)

	for _, fixture := range verificationFixturePaths(t) {
		data, err := disk.Read(filepath.Join(fixture, "quality-records.json"))
		if err != nil {
			t.Fatal(err)
		}

		var records []archivedVerificationControl

		err = json.Unmarshal(data, &records)
		if err != nil {
			t.Fatal(err)
		}

		controls := verificationFixtureControls(t, fixture)
		assertControlGroundTruth(t, controls)

		if len(records) != len(controls) {
			t.Fatal("incomplete archive", fixture)
		}

		for index, record := range records {
			assertArchivedVerificationControl(t, record, controls[index], expected, seen)
		}
	}

	if len(seen) != len(expected) {
		t.Fatalf("preserved %d controls, want %d", len(seen), len(expected))
	}
}

func TestVerificationControlCLIsPlantBodiesAndForwardVerification(t *testing.T) {
	utility := recordingTestUtility(t)
	shims, programs := t.TempDir(), t.TempDir()

	err := utility.Substitute(shims, []proxycli.Dependency{{Name: claude.Bin}, {Name: verificationCodex}})
	if err != nil {
		t.Fatal(err)
	}

	addRecordingGo(t, programs)
	writeRecordingProbe(t, filepath.Join(programs, claude.Bin), "#!/bin/sh\nprintf '<%s>\\n' \"$@\" >&2\ncat\nexit 7\n")

	err = os.Chmod(filepath.Join(programs, claude.Bin), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", programs+":/usr/bin:/bin")

	for _, fixture := range verificationFixturePaths(t) {
		absolute, err := filepath.Abs(fixture)
		if err != nil {
			t.Fatal(err)
		}

		env := proxycli.Environment("observe", shims, t.TempDir(), t.TempDir(), []string{shims})
		env[0] = "PATH=" + shims + ":" + absolute + "/cli:" + programs + ":/usr/bin:/bin"

		for _, control := range verificationFixtureControls(t, fixture) {
			t.Run(control.ID, func(t *testing.T) {
				assertPlantedVerificationControl(t, shims, env, control)
			})
			t.Run(control.ID+"/grouping", func(t *testing.T) {
				assertGroupedVerificationControl(t, shims, env, control)
			})
		}

		root := t.TempDir()
		input := "Verification evidence for one Concept Item.\n" +
			"Propose the step definition patterns of one concept for one suite,\n" +
			"Group the supplied distinct unbound steps of one suite and block.\n"
		args := []string{"--flag", "two words", ""}
		result := runVerificationControlCLI(t, shims, root, env, claude.Bin, args, input)

		want := "<" + strings.Join(args, ">\n<") + ">\n"
		if result.Code != 7 || result.Stdout != input || result.Stderr != want {
			t.Fatalf("verification was intercepted or changed: %+v", result)
		}
	}
}

func assertGroupedVerificationControl(t *testing.T, shims string, env []string, control fixtureVerificationControl) {
	t.Helper()

	protocol := "Group the supplied distinct unbound steps of one suite and block."
	args := []string{verificationSystemPromptFlag, protocol}
	input := fmt.Sprintf(`{"steps":[{"key":"4","block":%q,"text":"first"},{"key":"9","block":%q,"text":"second"}]}`,
		control.Block, control.Block)
	result := runVerificationControlCLI(t, shims, t.TempDir(), env, claude.Bin, args, input)

	var envelope struct {
		StructuredOutput any `json:"structured_output"`
	}

	err := json.Unmarshal([]byte(result.Stdout), &envelope)
	if err != nil || result.Code != 0 {
		t.Fatalf("grouping plant: %+v, %v", result, err)
	}

	var expected any

	err = json.Unmarshal(fmt.Appendf(nil, `{"assignments":[`+
		`{"index":4,"concept":%q,"score":10,"runner_up":null,"runner_up_score":0},`+
		`{"index":9,"concept":%q,"score":10,"runner_up":null,"runner_up_score":0}]}`,
		control.Concept, control.Concept), &expected)
	if err != nil || !reflect.DeepEqual(envelope.StructuredOutput, expected) {
		t.Fatalf("grouping must assign every batch key to its planted Concept: %+v, %v", result, err)
	}

	input = `{"steps":[{"key":"1","block":"uncontrolled","text":"forward me"}]}`

	result = runVerificationControlCLI(t, shims, t.TempDir(), env, claude.Bin, args, input)
	if result.Code != 7 || result.Stdout != input || result.Stderr != "<--system-prompt>\n<"+protocol+">\n" {
		t.Fatalf("uncontrolled grouping was intercepted or changed: %+v", result)
	}
}

func assertPlantedVerificationControl(t *testing.T, shims string, env []string, control fixtureVerificationControl) {
	t.Helper()
	root := t.TempDir()
	input := fmt.Sprintf(`{"block":%q,"concept":{"concept":%q}}`, control.Block, control.Concept)
	result := runVerificationControlCLI(t, shims, root, env, claude.Bin,
		[]string{verificationSystemPromptFlag, "Propose the step definition patterns of one concept for one suite,"}, input)

	var proposal struct {
		StructuredOutput struct {
			Patterns []struct {
				Pattern string `json:"pattern"`
			} `json:"patterns"`
		} `json:"structured_output"`
	}

	err := json.Unmarshal([]byte(result.Stdout), &proposal)
	if err != nil || result.Code != 0 || len(proposal.StructuredOutput.Patterns) != 1 ||
		proposal.StructuredOutput.Patterns[0].Pattern != control.Definition.Pattern {
		t.Fatalf("pattern plant: %+v, %v", result, err)
	}

	target := filepath.Join(root, control.Definition.File)
	writeRecordingProbe(t, target, "pending skeleton")

	body := fmt.Sprintf("Concept: %s (%s)\n=== FILE_START: tmp/result.yaml ===\n", control.Concept, control.Block)
	result = runVerificationControlCLI(t, shims, root, env, verificationCodex,
		[]string{"-o", filepath.Join(root, "answer.txt")}, body)

	written, err := disk.Read(target)
	if err != nil || result.Code != 0 || string(written) != control.Source {
		t.Fatalf("body plant: %+v, %v", result, err)
	}
}

func runVerificationControlCLI(t *testing.T, shims, root string, env []string, program string,
	args []string, input string) cli.Result {
	t.Helper()

	result, err := spec.Run(append([]string{filepath.Join(shims, program)}, args...), cli.Options{
		Dir: root, Env: cli.Inherit().Set(env...), Stdin: strings.NewReader(input), Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func archivedVerificationHashes() map[string]string {
	return map[string]string{
		"concept-bodies-extra-claude-003":           "393219c2d4344878fdab758c9574b66d7bbda72eb7e081312cb95d2fb48c98ba",
		"concept-bodies-extra-claude-005":           "f61c35abb6d2ad5e7d5061c5c7511b39199ae88634117dda7752c422703a9885",
		"concept-bodies-capture-claude-004":         "17a94556ba31e99bd8c434970cb497eecb85482326cde27f9f8703fe2f161b6d",
		"concept-bodies-capture-claude-008":         "e954c26cb20bac025c19884c7de87eaf92d8fd86eca6bee2afb6fee00e016f61",
		"concept-bodies-cases-claude-009":           "9152056557c44c988850d58970e71396d72173b95c38523493d6fa83cfaa6e2a",
		"concept-bodies-cases-claude-011":           "b24645c943a5118a7cb23e231deced085676a3072462e47489d059c534f593d3",
		"concept-bodies-contract-claude-003":        "52439b770f3a9b2e8363f96c87a81feb3c4ec9d1733a2600ae3453bf9555d619",
		"build-steps-critique-assertion-claude-005": "4ceec17194130bf1164410efd23ada38857a4de1f832ff1ef9ca553223675061",
		"build-steps-critique-assertion-claude-007": "7b5cf75d9f2c7c071d28eae2be1cbd0775a130875c9a602b785518ce7e5aaa57",
		"build-steps-critique-correct-claude-006":   "dc9f9f125987b3b959223d97d3efcb0ee687fec4b5e48a302ed95eae1b788b98",
		"computed-assertion-control":                "188ea0f5adbdab6bc50613fa7171f84cdb7bab140c098dbee0008be180cd3c79",
	}
}

func assertArchivedVerificationControl(t *testing.T, record archivedVerificationControl,
	control fixtureVerificationControl, expected map[string]string, seen map[string]bool) {
	t.Helper()

	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(record.Context+"\x00"+record.Sources+"\x00"+record.System)))
	if expected[record.Item] != digest || seen[record.Item] {
		t.Fatal("changed or duplicated archived control", record.Item)
	}

	seen[record.Item] = true

	source := strings.SplitN(record.Sources, "\n--- "+control.Definition.File+" ---\n", 2)
	if control.ID != record.Item || len(source) != 2 || strings.SplitN(source[1], "\n--- ", 2)[0] != control.Source {
		t.Fatal("the planted body differs from the preserved control", record.Item)
	}
}

func assertControlGroundTruth(t *testing.T, controls []fixtureVerificationControl) {
	t.Helper()

	data, err := disk.Read("../../services/true-bdd/internal/infrastructure/verification/testdata/verdicts.json")
	if err != nil {
		t.Fatal(err)
	}

	var verdicts []struct {
		Control  string `json:"control"`
		Expected string `json:"expected_verdict"`
		Category string `json:"category"`
	}

	err = json.Unmarshal(data, &verdicts)
	if err != nil {
		t.Fatal(err)
	}

	for _, control := range controls {
		found := false

		for _, verdict := range verdicts {
			if verdict.Control != control.ID {
				continue
			}

			found = true

			if verdict.Expected != control.Expected || verdict.Category != control.Category {
				t.Fatal("the planted control changes the settled expected verdict", control.ID)
			}
		}

		if !found {
			t.Fatal("unknown preserved control", control.ID)
		}
	}
}
