package shell_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

func TestWatchdog(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		script           string
		stall, timeLimit time.Duration
		want             error
	}{
		{"stall", "sleep 10", 30 * time.Millisecond, time.Second, shell.ErrStalled},
		{"total", "while :; do echo active; sleep 0.01; done", time.Second, 60 * time.Millisecond, shell.ErrTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			began := time.Now()

			_, err := shell.Watch(t.Context(), shArgv(test.script), shell.Options{},
				shell.Watchdog{Stall: test.stall, Limit: test.timeLimit, Grace: 20 * time.Millisecond})
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}

			if time.Since(began) > time.Second {
				t.Fatal("watchdog left child running")
			}
		})
	}
}

func TestWatchStopsLeftoverDescendants(t *testing.T) {
	t.Parallel()

	for _, script := range []string{
		`(trap '' TERM; sleep 0.3; echo leaked > "$LEAK") & exit 0`,
		`(trap '' TERM; sleep 0.3; echo leaked > "$LEAK") & sleep 10`,
	} {
		t.Run(script, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "leaked")
			opts := shell.Options{Env: shell.Inherit().Set("LEAK=" + path)}
			_, _ = shell.Watch(t.Context(), shArgv(script), opts,
				shell.Watchdog{Stall: 50 * time.Millisecond, Limit: time.Second, Grace: 20 * time.Millisecond})
			time.Sleep(350 * time.Millisecond)

			_, err := os.Stat(path)
			if err == nil {
				t.Fatal("descendant survived shutdown")
			}
		})
	}
}
