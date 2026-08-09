package main

import (
	_ "embed"
	"sort"
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

// roleOrder is the order roles appear in the summary tables — the order
// the engine runs them in, so the table reads as a sequence.
func roleOrder() []string {
	return []string{rolePrompt, roleFix, roleApply, roleJudge}
}

// Renderer accumulates the report document.
type Renderer struct {
	out     strings.Builder
	session string
	// indexPath names the file being written, so per-fixture links can
	// be derived from it rather than hardcoded.
	indexPath string

	fixtures []*Fixture
	turns    []*Turn

	totalWall     float64
	deterministic float64
	modelTime     float64
	engineCost    float64
	judgeCost     float64
	totalTokens   int
	maxTurn       float64
	bootTotal     float64
	passed        int
}

// NewRenderer precomputes every suite-level total the sections share, so
// no section has to walk the fixtures again to answer "out of what?".
func NewRenderer(fixtures []*Fixture, session, indexPath string) *Renderer {
	renderer := &Renderer{
		fixtures:  fixtures,
		session:   session,
		indexPath: indexPath,
		maxTurn:   1,
	}

	for _, fixture := range fixtures {
		renderer.totalWall += fixture.Wall.Seconds()
		renderer.engineCost += fixture.Cost
		renderer.totalTokens += fixture.Tokens

		if fixture.Verdict == verdictPass {
			renderer.passed++
		}

		if fixture.Judge != nil {
			renderer.judgeCost += fixture.Judge.CostUSD
			renderer.totalTokens += fixture.Judge.Tokens
		}

		for _, phase := range fixture.Phases {
			if phase.Kind == KindDeterministic {
				renderer.deterministic += phase.Seconds
			} else {
				renderer.modelTime += phase.Seconds
			}
		}

		renderer.collectTurns(fixture)
	}

	return renderer
}

// Render produces the whole document.
func (r *Renderer) Render() string {
	r.out.WriteString(documentHead)
	r.writeHeader()
	r.writeSummaryTiles()
	r.writeFindings()
	r.writeTimeline()
	r.writeGoSideBreakdown()
	r.writeTurnInternals()
	r.writeRoleTable()
	r.writeCLITable()
	r.writeTurnTrace()
	r.writeCaveats()
	r.out.WriteString("</main></body>")

	return r.out.String()
}

// collectTurns flattens one fixture's turns into the suite-wide list and
// updates the totals that depend on them.
func (r *Renderer) collectTurns(fixture *Fixture) {
	for _, turn := range fixture.Turns {
		r.turns = append(r.turns, turn)

		if seconds := turn.Seconds(); seconds > r.maxTurn {
			r.maxTurn = seconds
		}

		if split := turn.Split(); split.Known {
			r.bootTotal += split.Boot.Seconds()
		}
	}
}

// detailHref is the relative link from the index to one fixture's page.
// Relative so the pair can be copied or served from anywhere together.
func (r *Renderer) detailHref(fixture *Fixture) string {
	return detailFileName(r.indexPath, fixture.Name)
}

// write appends raw markup.
func (r *Renderer) write(parts ...string) {
	for _, part := range parts {
		r.out.WriteString(part)
	}
}

// bar renders a proportional bar in its track.
func bar(pct float64, color string, height int) string {
	return `<div class="bar" style="height:` + strconv.Itoa(height) + `px">` +
		`<span style="width:` + formatPercent(clampPercent(pct), percentDecimals) +
		`%;background:` + color + `"></span></div>`
}

// roleDot is a role name preceded by its color key.
func roleDot(role string) string {
	label := role
	if label == "" {
		label = emDash
	}

	return `<span class="dot" style="background:` + roleColor(role) + `"></span>` +
		escapeHTML(label)
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

// sortedKeys returns a map's keys in a stable order.
func sortedKeys[V any](source map[string]V) []string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
