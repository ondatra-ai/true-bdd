package gates

import (
	"fmt"
	"io"
	"time"
)

// Timing is how long one gate took. Only gates that ran have one; the summary
// pairs the list against the whole table to name what the selector skipped.
type Timing struct {
	Name    string
	Elapsed time.Duration
}

// RenderSummary prints every gate in table order — how long each took, or
// that the selector skipped it. A gate the diff did not need is as interesting
// as a slow one, which is the whole point of `--changed`.
func RenderSummary(out io.Writer, all, selected []Gate, timings []Timing) {
	elapsed := make(map[string]time.Duration, len(timings))
	for _, timing := range timings {
		elapsed[timing.Name] = timing.Elapsed
	}

	ran := make(map[string]bool, len(selected))
	for _, gate := range selected {
		ran[gate.Name] = true
	}

	_, _ = fmt.Fprintln(out, "\n| Gate | Seconds |")
	_, _ = fmt.Fprintln(out, "| --- | --- |")

	var total time.Duration

	for _, gate := range all {
		total += elapsed[gate.Name]
		_, _ = fmt.Fprintf(out, "| %s | %s |\n", gate.Name, verdict(gate, ran, elapsed))
	}

	_, _ = fmt.Fprintf(out, "| **total** | **%s** |\n\n", seconds(total))
}

// verdict is one row's right-hand column: a duration, "skipped", or "not
// reached" for a gate the first failure stopped the pipeline before.
func verdict(gate Gate, ran map[string]bool, elapsed map[string]time.Duration) string {
	switch {
	case !ran[gate.Name]:
		return "skipped"
	case elapsed[gate.Name] == 0:
		return "not reached"
	default:
		return seconds(elapsed[gate.Name])
	}
}

func seconds(elapsed time.Duration) string {
	return fmt.Sprintf("%.1f", elapsed.Seconds())
}
