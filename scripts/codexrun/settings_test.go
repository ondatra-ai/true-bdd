package codexrun_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

func TestWatchdogSettingsRejectNonfiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("CODEX_RUN_STALL_MINUTES", value)

			runner := codexrun.New(t.TempDir())
			if runner.Stall <= 0 {
				t.Fatalf("unbounded watchdog from %q", value)
			}
		})
	}
}
