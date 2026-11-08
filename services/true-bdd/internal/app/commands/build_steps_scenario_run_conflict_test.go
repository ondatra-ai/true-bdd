package commands_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestBuildStepsScenarioRunConflict(t *testing.T) {
	for _, fix := range []bool{false, true} {
		mode := scenarioRunBareMode
		if fix {
			mode = scenarioRunFixMode
		}

		t.Run(mode, func(t *testing.T) {
			deps, reply := scenarioRunHost(t, mode)
			replaceScenarioRunFile(t, scenarioRunStateFile, "Program string;", "ScenarioRun string; Program string;")
			before := scenarioRunHostFiles(t)
			log := captureScenarioRunLog(t)
			menu := strings.NewReader("1\n")
			deps.UserInputCollector = input.NewUserInputCollectorFrom(menu)

			err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", scenarioRunArchitecture, fix)
			if err == nil {
				t.Error("expected conflicting ScenarioRun field to stop the run")
			} else {
				for _, want := range []string{scenarioRunStateFile, "State", "ScenarioRun", "conflict"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("failure missing %q: %v", want, err)
					}
				}
			}

			if fix {
				requireScenarioRunCannotFix(t, err)
			}

			requireScenarioRunConflictVerdict(t, log.String())

			if menu.Len() != len("1\n") {
				t.Error("conflict reached the fix menu")
			}

			if len(reply.requests) != 0 {
				t.Errorf("unexpected AI turns: %v", reply.requests)
			}

			if !maps.Equal(before, scenarioRunHostFiles(t)) {
				t.Error("conflict changed host files")
			}
		})
	}
}

func requireScenarioRunConflictVerdict(t *testing.T, log string) {
	t.Helper()

	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, `msg="Check answered"`) && strings.Contains(line, "check=scenario-run") &&
			strings.Contains(line, "answer=fail") && strings.Contains(line, scenarioRunStateFile) &&
			strings.Contains(line, "ScenarioRun conflicts") {
			return
		}
	}

	t.Errorf("Q did not report the conflict: %s", log)
}

func requireScenarioRunCannotFix(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, pkgerrors.ErrCannotFix) {
		t.Errorf("expected ErrCannotFix: %v", err)
	}

	if err == nil {
		return
	}

	for _, want := range []string{"scenario-run #", "rename or remove", "build steps --fix"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("cannot-fix missing %q: %v", want, err)
		}
	}
}
