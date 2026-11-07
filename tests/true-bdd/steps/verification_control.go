package steps

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

type verificationControl struct {
	ID         string `json:"id"`
	Expected   string `json:"expected_verdict"`
	Category   string `json:"category"`
	Definition struct {
		Function string `json:"function"`
	} `json:"definition"`
}

func registerVerificationControlSteps(suite testkit.StepRegistrar[State]) {
	suite.Step(`^the verification controls in "([^"]+)" use preserved pattern proposals and bodies$`,
		useVerificationControls)
	suite.Step(`^every preserved control has its expected step-verification verdict$`, assertVerificationControls)
}

//truebdd:concept given "fixture AI CLIs"
func useVerificationControls(state *State, args []string) error {
	sanctioned := []string{"verification-controls-extra-original", "verification-controls-extra-repair",
		"verification-controls-capture-original", "verification-controls-capture-repair",
		"verification-controls-operation", "verification-controls-output",
		"verification-controls-toolchain", "verification-controls-assertion-original",
		"verification-controls-assertion-repair",
		"verification-controls-computed"}
	if state.Fixture.Name != args[0] || !slices.Contains(sanctioned, args[0]) {
		return state.fail("preserved verification controls are not sanctioned for %q", state.Fixture.Name)
	}

	state.SetEnv("CODEX_HOME=" + filepath.Join(state.Fixture.Dir, "auth"))

	for _, program := range []string{"claude", "codex"} {
		err := useFixtureCLI(state, "cli", program)
		if err != nil {
			return err
		}
	}

	return nil
}

//truebdd:concept then "file content"
func assertVerificationControls(state *State, _ []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	data, err := testkit.Read(filepath.Join(state.Fixture.Dir, "controls.json"))
	if err != nil {
		return state.fail("read preserved controls: %w", err)
	}

	var controls []verificationControl

	err = json.Unmarshal(data, &controls)
	if err != nil {
		return state.fail("decode preserved controls: %w", err)
	}

	prompts, err := filepath.Glob(filepath.Join(state.Result.TmpDir,
		"tmp", "*", "*checklist-build-steps-step-verification-user.txt"))
	if err != nil {
		return state.fail("find saved verification prompts: %w", err)
	}

	for _, control := range controls {
		err = assertControlVerdicts(state, control, prompts)
		if err != nil {
			return err
		}
	}

	if len(controls) == 0 {
		return state.fail("fixture has no preserved controls")
	}

	return nil
}

func assertControlVerdicts(state *State, control verificationControl, prompts []string) error {
	seen := false

	for _, prompt := range prompts {
		source, err := testkit.Read(prompt)
		if err != nil {
			return state.fail("read control prompt: %w", err)
		}

		if !strings.Contains(string(source), "function: "+control.Definition.Function) {
			continue
		}

		seen = true

		err = assertControlAnswer(state, control, prompt)
		if err != nil {
			return err
		}
	}

	if !seen {
		return state.fail("control %s has no saved verification Q; inspect %s/tmp", control.ID, state.Result.TmpDir)
	}

	return nil
}

func assertControlAnswer(state *State, control verificationControl, prompt string) error {
	response := strings.TrimSuffix(prompt, "-user.txt") + "-response.txt"

	data, err := testkit.Read(response)
	if err != nil {
		return state.fail("control %s has no answer; saved prompt %s: %w", control.ID, prompt, err)
	}

	var answer struct {
		Answer   string `json:"answer"`
		Findings []struct {
			Category string `json:"category"`
		} `json:"findings"`
	}

	err = json.Unmarshal(data, &answer)
	if err != nil {
		return state.fail("decode control %s answer %s: %w", control.ID, response, err)
	}

	categoryMatches := control.Category == ""
	for _, finding := range answer.Findings {
		categoryMatches = categoryMatches || finding.Category == control.Category
	}

	if answer.Answer != control.Expected || !categoryMatches {
		return state.fail("control %s expected %s:%s, got %s; saved prompt %s; answer %s",
			control.ID, control.Expected, control.Category, data, prompt, response)
	}

	return nil
}
