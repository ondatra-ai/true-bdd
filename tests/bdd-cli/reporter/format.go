package reporter

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	secondsPerMinute = 60.0
	millisPerSecond  = 1000.0
	percentScale     = 100.0
	// thousandsGroup is the digit-grouping width for the "1,234,567"
	// separators the summary tables use.
	thousandsGroup = 3
	// emDash stands in for every absent number, so a missing measurement
	// never renders as a confident 0.
	emDash = "—"
)

// formatDuration renders a span the way the tables read best: sub-second
// values in milliseconds (an engine step is meaningless as "0.0s"),
// then seconds, then minutes once the number stops being scannable.
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

// formatElapsed renders when a slice began, measured from the start of
// the fixture's wall clock. The first slice reads "start" rather than
// "0ms", because a zero here means the beginning of the run and not a
// measurement that rounded away.
func formatElapsed(seconds float64) string {
	if seconds < 1/millisPerSecond {
		return "start"
	}

	return "+" + formatSeconds(seconds)
}

// formatMoney renders a dollar amount to four places — turn costs run to
// fractions of a cent, and rounding them to two hides the cheap turns
// entirely. Zero renders as absent, because every zero in this report
// means "the CLI reported nothing", not "this was free".
func formatMoney(value float64) string {
	if value == 0 {
		return emDash
	}

	return "$" + groupThousands(strconv.FormatFloat(value, 'f', 4, 64))
}

// formatCount renders a token count with thousands separators, and an
// absent marker for zero — same reasoning as formatMoney.
func formatCount(value int) string {
	if value == 0 {
		return emDash
	}

	return groupThousands(strconv.Itoa(value))
}

// formatPercent renders a percentage to the given number of decimals.
func formatPercent(value float64, decimals int) string {
	return strconv.FormatFloat(value, 'f', decimals, 64)
}

// groupThousands inserts commas into the integer part of a decimal
// string, leaving any fractional part and sign untouched.
func groupThousands(number string) string {
	sign := ""
	if strings.HasPrefix(number, "-") {
		sign, number = "-", number[1:]
	}

	whole, fraction, hasFraction := strings.Cut(number, ".")

	var grouped strings.Builder

	for index, digit := range whole {
		if index > 0 && (len(whole)-index)%thousandsGroup == 0 {
			grouped.WriteByte(',')
		}

		grouped.WriteRune(digit)
	}

	out := sign + grouped.String()
	if hasFraction {
		out += "." + fraction
	}

	return out
}

// escapeHTML escapes text for HTML body and attribute contexts.
//
// Hand-rolled rather than html.EscapeString because the report is
// diffed against the reference renderer output during the port, and the
// two disagree on entity spelling for quotes ("&#34;" vs "&quot;").
// Same safety, stable bytes.
func escapeHTML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#x27;",
	)

	return replacer.Replace(text)
}

// clampPercent keeps a computed share inside a bar's drawable range, so
// a rounding overshoot cannot render a bar wider than its track.
func clampPercent(pct float64) float64 {
	if pct < 0 {
		return 0
	}

	if pct > percentScale {
		return percentScale
	}

	return pct
}

// shareOf is pct = part/whole, guarding the empty-run case where every
// total is zero and the division would be NaN.
func shareOf(part, whole float64) float64 {
	if whole == 0 {
		return 0
	}

	return part / whole * percentScale
}
