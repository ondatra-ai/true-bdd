package report

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	secondsPerMinute = 60.0
	millisPerSecond  = 1000.0
	// emDash stands in for every absent number, so a missing measurement never
	// renders as a confident 0.
	emDash = "—"
)

// Table renders the run as the operation / sub-operation table: one row per
// node, its depth carried by the number rather than by indentation, so a tree
// of any depth still reads as a table.
func (r *Run) Table() string {
	var out strings.Builder

	fmt.Fprintf(&out, "## %s — run %s\n\n", r.Tool, r.ID)
	out.WriteString("| # | Operation | Status | Time |\n")
	out.WriteString("|---|---|---|---|\n")

	for index, node := range r.Nodes {
		writeRows(&out, node, strconv.Itoa(index+1))
	}

	fmt.Fprintf(&out, "| | **%s** | **%s** | **%s** |\n",
		cell(r.Tool), r.Status, formatDuration(r.Duration, true))

	return out.String()
}

// Narrate puts the table on the log a line at a time. One record carrying
// newlines is escaped into a single unreadable line by slog's text handler,
// and this table exists to be read.
func (r *Run) Narrate() {
	for line := range strings.SplitSeq(strings.TrimRight(r.Table(), "\n"), "\n") {
		slog.Info(line)
	}
}

func writeRows(out *strings.Builder, node *Node, number string) {
	// A skipped stage has a real duration and it is meaningless: the markers
	// are all that ran. An unclosed node has no end to measure against.
	timed := node.Measured && node.Status != StatusSkipped

	fmt.Fprintf(out, "| %s | %s | %s | %s |\n",
		number, cell(node.Name), node.Status, formatDuration(node.Duration, timed))

	for index, child := range node.Children {
		writeRows(out, child, number+"."+strconv.Itoa(index+1))
	}
}

// cell keeps a name that contains a pipe from ending the column early.
func cell(name string) string {
	return strings.ReplaceAll(name, "|", `\|`)
}

// formatDuration renders a span the way a reader scans it: sub-second values
// in milliseconds, then seconds, then minutes.
func formatDuration(span time.Duration, measured bool) string {
	if !measured {
		return emDash
	}

	seconds := span.Seconds()

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
