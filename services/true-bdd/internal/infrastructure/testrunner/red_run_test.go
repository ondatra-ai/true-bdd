package testrunner_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner/dto"
)

const redFailureAction = "fail"
const redPackage = "host/calc"

func TestRedRunRequiresEachScenarioToFailInAnExecutedDefinition(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name, action, output string
		red, green           bool
	}{
		{"step failure", redFailureAction, "calc-001\n  step 2/3 failed: When double\n  missing executable", true, false},
		{"bring-up", redFailureAction, "bring the suite up: missing service", false, false},
		{"undefined step", redFailureAction, "undefined step: double", false, false},
		{"state constructor", redFailureAction, "building scenario state: missing service", false, false},
		{"green", "pass", "", false, true},
		{"skipped Scenario", "skip", "Scenario skipped", false, false},
		{"missing", "", "", false, false},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()
			report := redReport(t, example.action, example.output)

			verdict := testrunner.AssessRedRun([]byte(report), []string{"host/calc::TestCalc001", "host/peer::TestCalc001"})
			if (len(verdict.Red) == 1) != example.red || (len(verdict.Green) == 1) != example.green {
				t.Fatalf("verdict: %+v", verdict)
			}

			if len(verdict.Invalid) == 0 || !strings.Contains(strings.Join(verdict.Invalid, "\n"), "host/peer::TestCalc001") {
				t.Fatalf("missing peer Scenario must be named: %+v", verdict)
			}

			invalid := strings.Join(verdict.Invalid, "\n")
			if !example.red && !example.green && !strings.Contains(invalid, "host/calc::TestCalc001") {
				t.Fatalf("invalid red must name the Scenario: %+v", verdict)
			}
		})
	}
}

func redReport(t *testing.T, action, output string) string {
	t.Helper()

	var report strings.Builder

	for _, event := range []dto.GoTestEvent{
		{Action: "output", Package: redPackage, Test: "TestCalc001", Output: output},
		{Action: action, Package: redPackage, Test: "TestCalc001"},
		{Action: redFailureAction, Package: redPackage},
		{Action: redFailureAction, Package: "host/peer", FailedBuild: "host/service"},
	} {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}

		report.Write(encoded)
		report.WriteByte('\n')
	}

	return report.String()
}
