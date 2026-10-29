package runner_test

import (
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func TestBinaryBuildWaitsForStepAndSharesFailure(t *testing.T) {
	t.Parallel()

	builds := 0
	missing := errors.New("service directory absent")
	binary := runner.NewLazyBinary(func() (string, error) {
		builds++

		return "", missing
	})

	if builds != 0 {
		t.Fatal("building the harness must not build its service")
	}

	for range 2 {
		_, err := binary.Path()
		if !errors.Is(err, missing) {
			t.Fatalf("each Scenario needs its own step failure: %v", err)
		}
	}

	if builds != 1 {
		t.Fatalf("builds=%d, want one shared build result", builds)
	}
}
