package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestStartedCodexFailuresRerunOnlyBuildCode(t *testing.T) {
	for _, command := range []string{buildCodeCommand, buildTestsCommand, usApplyCommand} {
		for _, outcome := range []pkgerrors.CodexTurnOutcome{pkgerrors.CodexTurnNonZeroExit, pkgerrors.CodexTurnEmptyOutput} {
			t.Run(command+"/"+string(outcome), func(t *testing.T) {
				failure := &pkgerrors.CodexTurnError{Outcome: outcome, Cause: errors.New("writer failed")}
				spec := confirmationSpec(t, command, applyAnswer{err: failure})
				reruns := 0
				spec.PostNotApplied = func(context.Context, bool) (bool, error) {
					reruns++

					return true, nil
				}

				err := runner.Run(t.Context(), spec)
				if command == buildCodeCommand {
					if err != nil || reruns != 1 {
						t.Fatalf("Run=%v reruns=%d", err, reruns)
					}
				} else if !errors.Is(err, failure) || reruns != 0 {
					t.Fatalf("Run=%v reruns=%d", err, reruns)
				}
			})
		}
	}
}
