package codexrun_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

func TestNewestWeeklyWindow(t *testing.T) {
	for _, test := range []struct {
		name, window string
		want         string
	}{
		{"primary", `"primary":{"window_minutes":10080,"used_percent":91,"resets_at":%d}`, "91%% used"},
		{"expired", `"secondary":{"window_minutes":10080,"used_percent":91,"resets_at":1}`, "unknown"},
		{"missing-percent", `"primary":{"window_minutes":10080,"resets_at":%d}`, "unknown"},
		{"other-service", `"limit_id":"other",` +
			`"primary":{"window_minutes":10080,"used_percent":99,"resets_at":%d}`, "10%% used"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			reset := time.Now().Add(time.Hour).Unix()
			olderWindow := fmt.Sprintf(`"secondary":{"window_minutes":10080,"used_percent":10,"resets_at":%d}`, reset)
			older := weeklyEvent(time.Now().Add(-time.Hour), olderWindow)
			window := strings.ReplaceAll(test.window, "%d", strconv.FormatInt(reset, 10))
			newer := weeklyEvent(time.Now(), window)
			// Filename order and mtime cannot override the event timestamps.
			write(t, filepath.Join(root, "home/sessions/rollout-z.jsonl"), older)
			write(t, filepath.Join(root, "home/sessions/nested/rollout-a.jsonl"), newer+"\n{partial")
			record, err := runner.Start(t.Context(), opts)
			assertStatus(t, record, err, doneStatus)

			want := strings.ReplaceAll(test.want, "%%", "%")
			if !strings.Contains(record.Summary(), "weekly limit: "+want) {
				t.Fatal(record.Summary())
			}
		})
	}
}

func weeklyEvent(at time.Time, window string) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","rate_limits":{%s}}}`,
		at.UTC().Format(time.RFC3339Nano), window)
}
