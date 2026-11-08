package reporter

import "testing"

// TestFormatDuration pins the scale switches. A step that renders as
// "0.0s" instead of "19ms" reads as "free", which is the opposite of
// what the report exists to show.
func TestFormatDuration(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		present bool
		want    string
	}{
		{"absent", 0, false, emDash},
		{"sub-second stays in ms", 0.0136, true, "14ms"},
		{"zero is a real measurement", 0, true, "0ms"},
		{"seconds to one place", 10.02, true, "10.0s"},
		{"just under a minute", 59.94, true, "59.9s"},
		{"minute boundary", 60, true, "1m 00.0s"},
		{"minutes zero-pad the seconds", 122.623, true, "2m 02.6s"},
		{"long run", 205.88, true, "3m 25.9s"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatDuration(testCase.seconds, testCase.present)
			if got != testCase.want {
				t.Errorf("formatDuration(%v, %v) = %q, want %q",
					testCase.seconds, testCase.present, got, testCase.want)
			}
		})
	}
}
