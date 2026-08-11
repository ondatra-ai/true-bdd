package reporter

import (
	_ "embed"
	"strconv"
	"strings"
)

// documentHead is the page shell and stylesheet: doctype through the
// opening <main>. Kept as an asset so the CSS can be edited without
// touching Go, and so the renderer below stays about structure.
//
//go:embed head.html
var documentHead string

// Palette. The three role colors are a validated colorblind-safe
// triple; every role is also labeled in text, so color is never
// load-bearing on its own.
const (
	colorPrompt        = "#2563eb"
	colorFix           = "#ea580c"
	colorApply         = "#059669"
	colorJudge         = "#7c3aed"
	colorDeterministic = "#94a3b8"
	colorBoot          = "#64748b"
	colorTeardown      = "#cbd5e1"
	colorFallback      = "#64748b"
)

// The three checklist roles, plus the harness's own verdict call.
// roleJudge is not an engine turn — it is recovered from the test
// process's records — but it spends real money, so it is carried
// alongside the others everywhere costs are totalled.
const (
	rolePrompt = "prompt"
	roleFix    = "fix"
	roleApply  = "apply"
	roleJudge  = "judge"
)

// minGanttWidth keeps a millisecond-scale slice visible on a gantt.
// Without it the engine's own work renders as nothing at all.
const minGanttWidth = 0.4

// percentDecimals is the precision CSS widths are written to. Two
// places keeps a sub-percent slice from collapsing to 0% wide.
const percentDecimals = 2

// The verdicts `go test` reports for a subtest.
const (
	verdictPass = "PASS"
	verdictFail = "FAIL"
)

// The three agent CLIs the engine can route a turn to.
const (
	cliClaude = "claude"
	cliCrush  = "crush"
	cliCodex  = "codex"
)

// roleColor maps a checklist role to its bar color.
func roleColor(role string) string {
	switch role {
	case rolePrompt:
		return colorPrompt
	case roleFix:
		return colorFix
	case roleApply:
		return colorApply
	case roleJudge:
		return colorJudge
	default:
		return colorFallback
	}
}

// Renderer accumulates the report document.
type Renderer struct {
	out     strings.Builder
	session string

	fixtures []*Fixture
	turns    []*Turn

	totalWall  float64
	engineCost float64
	judgeCost  float64
	passed     int
}

// newRenderer precomputes the run-level totals the summary reports.
//
// Every one of them is a straight sum over something a log or a harness
// record measured — no phase model, no attribution of the spans between
// records. The index used to headline deterministic-versus-model splits
// derived that way; the detail pages still carry them, where the reader
// has opted in.
func newRenderer(fixtures []*Fixture, session string) *Renderer {
	renderer := &Renderer{
		fixtures: fixtures,
		session:  session,
	}

	for _, fixture := range fixtures {
		renderer.totalWall += fixture.Wall.Seconds()
		renderer.engineCost += fixture.Cost

		if fixture.Verdict == verdictPass {
			renderer.passed++
		}

		if fixture.Judge != nil {
			renderer.judgeCost += fixture.Judge.CostUSD
		}

		renderer.turns = append(renderer.turns, fixture.Turns...)
	}

	return renderer
}

// Render produces the whole document.
func (r *Renderer) Render() string {
	r.out.WriteString(documentHead)
	r.writeHeader()
	r.writeFixtureList()
	r.writeRunSummary()
	r.out.WriteString("</main></body>")

	return r.out.String()
}

// detailHref is the relative link from the index to one fixture's page.
// Relative so the pair can be copied or served from anywhere together.
func (r *Renderer) detailHref(fixture *Fixture) string {
	return detailFileName(fixture.Name)
}

// write appends raw markup.
func (r *Renderer) write(parts ...string) {
	for _, part := range parts {
		r.out.WriteString(part)
	}
}

// ganttBar is a slice placed on the run's wall clock: the fill starts
// where the slice began and is as wide as it lasted.
//
// The offset is what makes it a timeline. A plain proportional bar,
// every row hugging the left edge, answers "how long" but never "what
// came after what" — and renders a millisecond slice as nothing at all.
func ganttBar(offsetPct, widthPct float64, color string) string {
	if widthPct < minGanttWidth {
		widthPct = minGanttWidth
	}

	return `<div class="gantt"><span style="margin-left:` +
		formatPercent(clampPercent(offsetPct), percentDecimals) +
		`%;width:` + formatPercent(clampPercent(widthPct), percentDecimals) +
		`%;background:` + color + `"></span></div>`
}

// verdictChip renders a fixture's PASS/FAIL badge.
func verdictChip(verdict string) string {
	switch verdict {
	case verdictPass:
		return `<span class="chip pass">PASS</span>`
	case verdictFail:
		return `<span class="chip fail">FAIL</span>`
	default:
		return `<span class="chip none">` + emDash + `</span>`
	}
}

// phaseColor is the bar color for a timeline slice.
func phaseColor(phase Phase) string {
	if phase.Kind == KindDeterministic {
		return colorDeterministic
	}

	return roleColor(phase.Role)
}

// itoa is strconv.Itoa under a short name, for the many inline uses.
func itoa(value int) string {
	return strconv.Itoa(value)
}
