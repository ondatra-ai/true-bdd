package steps

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const modelPatternMatches = "matches"

type checklistPromptRule struct {
	Glob      string `yaml:"glob"`
	Pattern   string `yaml:"pattern"`
	Operator  string `yaml:"operator"`
	Checklist string `yaml:"checklist"`
}

func registerChecklistModelSteps(suite testkit.StepRegistrar[State]) {
	suite.Step(`^the (us_refine_checklist|us_apply_checklist|scen_check_checklist) prompt (.+)$`, assertChecklistModelRule)
	suite.Step(`^the (us_refine_checklist|us_apply_checklist|`+
		`scen_check_checklist|build_steps_checklist) Model scheme is shipped$`,
		assertModelSchemeShipped)
}

//truebdd:concept then "file content"
func assertChecklistModelRule(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	content, err := testkit.Read(filepath.Join(state.Fixture.Dir, "model-expectations.yaml"))
	if err != nil {
		return state.fail("read Model expectations: %w", err)
	}

	var rules map[string]checklistPromptRule

	err = yaml.Unmarshal(content, &rules)
	if err != nil {
		return state.fail("decode Model expectations: %w", err)
	}

	text := "the " + args[0] + " prompt " + args[1]

	rule, ok := rules[text]
	if !ok || rule.Glob == "" || rule.Pattern == "" ||
		(rule.Operator != modelPatternMatches && rule.Operator != "does not match") {
		return state.fail("missing or invalid Model expectation for %q", text)
	}

	if rule.Checklist != "" {
		return assertChecklistQuestionAndFix(state, rule)
	}

	return assertEveryFileMatches(state, []string{rule.Glob, rule.Operator, rule.Pattern})
}
