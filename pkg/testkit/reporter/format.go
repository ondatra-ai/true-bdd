package reporter

import (
	"fmt"
	"strconv"
)

const (
	secondsPerMinute = 60.0
	millisPerSecond  = 1000.0
	// emDash stands in for every absent number, so a missing measurement
	// never renders as a confident 0.
	emDash = "—"
)

// formatDuration renders a span the way a reader scans it: sub-second
// values in milliseconds, then seconds, then minutes. Still used because
// phase.Detail's prose has pinned wording (TestDiscoveryBoundCountsOnlyDiscoveryRuns).
func formatDuration(seconds float64, ok bool) string {
	if !ok {
		return emDash
	}

	if seconds < 1 {
		return strconv.FormatFloat(seconds*millisPerSecond, 'f', 0, 64) + "ms"
	}

	if seconds < secondsPerMinute {
		return strconv.FormatFloat(seconds, 'f', 1, 64) + "s"
	}

	minutes := int(seconds / secondsPerMinute)
	rest := seconds - float64(minutes)*secondsPerMinute

	return fmt.Sprintf("%dm %04.1fs", minutes, rest)
}

// formatSeconds is formatDuration for a value that is always present.
func formatSeconds(seconds float64) string {
	return formatDuration(seconds, true)
}
