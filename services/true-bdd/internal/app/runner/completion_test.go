package runner_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/logging"
)

func TestCompletionFollowsSuccessfulFinalize(t *testing.T) {
	for command, banner := range map[string]string{
		usRefineCommand: "US REFINE COMPLETE", usApplyCommand: "US APPLY COMPLETE",
		scenCheckCommand: "SCEN CHECK COMPLETE", buildCodeCommand: "BUILD CODE COMPLETE",
		buildTestsCommand: "BUILD TESTS COMPLETE",
	} {
		for _, outcome := range []string{"success", "failure", "deferred"} {
			t.Run(command+"/"+outcome, func(t *testing.T) {
				spec := confirmationSpec(t, command, applyAnswer{})
				spec.LoadItems = func(context.Context) ([]bool, error) { return nil, nil }
				spec.DeferCompletion = outcome == "deferred"

				var captured bytes.Buffer

				original := slog.Default()

				slog.SetDefault(slog.New(logging.Handler(&captured, "")))
				t.Cleanup(func() { slog.SetDefault(original) })

				var finalErr error
				if outcome == "failure" {
					finalErr = errors.New("finalize failed")
				}

				finalized := false
				spec.Finalize = func(*engine.Result[bool]) error {
					finalized = true

					if strings.Contains(captured.String(), banner) {
						t.Error("completion announced before Finalize returned")
					}

					return finalErr
				}

				err := runner.Run(t.Context(), spec)
				if !finalized || !errors.Is(err, finalErr) {
					t.Fatalf("Run = %v, finalized = %t; want %v", err, finalized, finalErr)
				}

				wantCount := 0
				if outcome == "success" {
					wantCount = 1
				}

				if got := strings.Count(captured.String(), banner); got != wantCount {
					t.Errorf("completion count = %d, want %d", got, wantCount)
				}
			})
		}
	}
}
