package steps

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	modelPatternMatches        = "matches"
	modelStoryPromptPattern    = "tmp/*/01-doc-summary-checklist-*-user.txt"
	modelScenarioPromptPattern = "tmp/*/01-E2E-001-checklist-*-user.txt"
)

type checklistPromptRule struct {
	glob     string
	pattern  string
	operator string
}

func registerChecklistModelSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the (us_refine_checklist|us_apply_checklist|scen_check_checklist) prompt (.+)$`, assertChecklistModelRule)
	suite.Step(`^the (us_refine_checklist|us_apply_checklist|`+
		`scen_check_checklist|build_steps_checklist) Model scheme is shipped$`,
		assertModelSchemeShipped)
}

//truebdd:concept then "file content"
func assertChecklistModelRule(state *State, args []string) error {
	if args[1] == "holds the checklist's question and fix verbatim" {
		return assertChecklistQuestionAndFix(state, args[0])
	}

	rule, ok := checklistModelRules()[args[0]+" "+args[1]]
	if !ok {
		return state.fail("unknown Model construction rule %q", args)
	}

	return assertEveryFileMatches(state, []string{rule.glob, rule.operator, rule.pattern})
}

func checklistModelRules() map[string]checklistPromptRule {
	return map[string]checklistPromptRule{
		"us_apply_checklist keeps the and modifier of the criterion's steps": {
			glob: "tmp/*/01-95.1-001-mcp-initialize-checklist-*-user.txt",
			pattern: "### Steps\\nGiven:\\n  - the MCP server is running on its configured port\\n  - the ser" +
				"ver accepts JSON-RPC over HTTP POST at /mcp\\nWhen:\\n  - the Claude User posts a vali" +
				"d JSON-RPC initialize request to /mcp\\nThen:\\n  - the server returns HTTP 200\\n  - a" +
				"nd the response body is a well-formed JSON-RPC envelope referencing the request id\\n" +
				"\\n\\n---",
			operator: modelPatternMatches,
		},
		"us_apply_checklist selects the criterion by id and sets lineage to StoryID-ACID": {
			glob: "tmp/*/01-mcp-endpoint-002-get-refused-checklist-*-user.txt",
			pattern: "\\*\\*Story ID:\\*\\* mcp-endpoint\\n\\*\\*Story Path:\\*\\* docs/product/stories/mcp-endpoin" +
				"t-requests\\.yaml\\n\\*\\*AC ID:\\*\\* 002-get-refused\\n\\*\\*Lineage Scenario ID:\\*\\* mcp-e" +
				"ndpoint-002-get-refused\\n\\*\\*Description:\\*\\* The MCP server must respond with HTTP " +
				"405 Method Not Allowed when a Claude User sends a GET request to the /mcp endpoint\\." +
				"\\n",
			operator: modelPatternMatches,
		},
		"us_apply_checklist keeps the other criterion's id, lineage and description": {
			glob: "tmp/*/01-mcp-endpoint-001-initialize-ok-checklist-*-user.txt",
			pattern: "\\*\\*AC ID:\\*\\* 001-initialize-ok\\n\\*\\*Lineage Scenario ID:\\*\\* mcp-endpoint-001-init" +
				"ialize-ok\\n\\*\\*Description:\\*\\* The MCP server must respond with a 200 status",
			operator: modelPatternMatches,
		},
		"us_apply_checklist groups Given, When, Then in source order with and/but modifiers": {
			glob: "tmp/*/*-mcp-endpoint-007-ordered-checklist-*-user.txt",
			pattern: "### Steps\\nGiven:\\n  - the MCP server is running on its configured port\\n  - and the" +
				" greeting is \"Hello\"\\n  - but no request has been sent\\nWhen:\\n  - the Claude User o" +
				"pens a connection\\n  - and the Claude User sends an HTTP GET request to /mcp\\nThen:\\" +
				"n  - the server returns HTTP 405\\n  - and the greeting is still \"Hello\"\\n  - but no " +
				"response body is printed\\n\\n\\n---",
			operator: modelPatternMatches,
		},
		"us_refine_checklist embeds the Story without a wrapper and keeps its Feature": {
			glob: modelStoryPromptPattern,
			pattern: "## User Story\\n```yaml\\nid: doc-summary\\nfeature: document-summaries\\ntitle: Documen" +
				"t Summary On Demand\\n",
			operator: modelPatternMatches,
		},
		"us_refine_checklist has no story wrapper": {
			glob:     modelStoryPromptPattern,
			pattern:  "(?m)^story:",
			operator: "does not match",
		},
		"scen_check_checklist keeps the Scenario's Registry key and fields": {
			glob: "tmp/*/06-E2E-001-checklist-*-user.txt",
			pattern: "\\*\\*Scenario ID:\\*\\* E2E-001\\n\\*\\*Description:\\*\\* The greeter must print the config" +
				"ured greeting followed by the name it is given, and exit with code 0\\n\\*\\*Service:\\*" +
				"\\* greeter\\n\\*\\*Generated test file:\\*\\* tests/greeter/greet_test\\.go\\n",
			operator: modelPatternMatches,
		},
		"scen_check_checklist keeps the Scenario's and modifier and lineage": {
			glob: "tmp/*/06-E2E-001-checklist-*-user.txt",
			pattern: "Then:\\n  - the command exits with code 0\\n  - and stdout matches \\^Hello Ada\\$\\n\\n##" +
				"# Lineage — user_stories\\[\\]\\n  - docs/product/stories/greet-by-name\\.yaml \\(greet-b" +
				"y-name-001-prints-greeting\\)\\n\\n---",
			operator: modelPatternMatches,
		},
		"scen_check_checklist does not mention merge_date": {
			glob:     "tmp/*/*-E2E-001-checklist-*-user.txt",
			pattern:  "merge_date",
			operator: "does not match",
		},
		"scen_check_checklist uses the Scenario's Registry key": {
			glob:     modelScenarioPromptPattern,
			pattern:  "\\*\\*Scenario ID:\\*\\* E2E-001\\n",
			operator: modelPatternMatches,
		},
		"scen_check_checklist keeps Scenario modifiers and lineage in source order": {
			glob: modelScenarioPromptPattern,
			pattern: "Then:\\n  - the command exits with code 0\\n  - and stdout matches \\^Hello Ada\\$\\n  - " +
				"but stderr has no lines\\n\\n### Lineage — user_stories\\[\\]\\n  - docs/product/stories/" +
				"z-greeting\\.yaml \\(z-greeting-002-name\\)\\n  - docs/product/stories/a-greeting\\.yaml " +
				"\\(a-greeting-001-name\\)\\n\\n---",
			operator: modelPatternMatches,
		},
	}
}
