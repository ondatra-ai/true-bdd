package reporter

// minGanttWidth keeps a millisecond-scale slice visible on the gantt.
// Without it the engine's own work — the most interesting number in the
// report — renders as nothing at all.
const minGanttWidth = 0.4

// writeHeader writes the page title and the run's one-line identity.
func (r *Renderer) writeHeader() {
	total := itoa(len(r.fixtures))

	r.write("\n<header>\n  <h1>TrueBDD fixture suite — run report</h1>\n",
		`  <p class="sub">Session <code>`, escapeHTML(r.session), "</code> ·\n     ",
		total, " fixture(s) with engine logs · ", itoa(r.passed), "/", total,
		" passed</p>\n</header>\n")
}

// writeSummaryTiles writes the five headline numbers.
func (r *Renderer) writeSummaryTiles() {
	deterministicPct := shareOf(r.deterministic, r.totalWall)

	wall := emDash
	if r.totalWall > 0 {
		wall = formatSeconds(r.totalWall)
	}

	// The three time tiles are exact and therefore do NOT sum to the
	// wall clock — the mixed remainder is named on the wall-clock tile
	// rather than folded into whichever neighbour is convenient.
	wallNote := "measured by go test"
	if r.mixedTime > 0 {
		wallNote += " · incl. " + formatSeconds(r.mixedTime) + " mixed"
	}

	tiles := [][3]string{
		{"Wall clock", wall, wallNote},
		{
			"Non-deterministic", formatSeconds(r.modelTime),
			formatPercent(shareOf(r.modelTime, r.totalWall), 1) + "% — models deciding",
		},
		{
			"Deterministic", formatSeconds(r.deterministic),
			formatPercent(deterministicPct, 1) + "% — engine + harness code",
		},
		{
			"Cost", formatMoney(r.engineCost + r.judgeCost),
			formatMoney(r.engineCost) + " engine + " + formatMoney(r.judgeCost) + " judge",
		},
		{"Tokens", formatCount(r.totalTokens), "in + out + cache"},
	}

	r.write(`<div class="tiles">`)

	for _, tile := range tiles {
		r.write(`<div class="tile"><div class="k">`, escapeHTML(tile[0]),
			`</div><div class="v">`, escapeHTML(tile[1]),
			`</div><div class="n">`, escapeHTML(tile[2]), `</div></div>`)
	}

	r.write("</div>")
}

// writeTimeline writes the report's spine: every slice of every
// fixture's wall clock, in order, tagged by what governs its duration.
func (r *Renderer) writeTimeline() {
	r.write(`<section><h2>Deterministic vs non-deterministic</h2>`,
		`<p class="lede">Every slice of the fixture's wall clock, in order, `,
		"tagged by what governs its duration. <strong>Deterministic</strong> ",
		"is Go code — engine start-up, test discovery, prompt rendering, ",
		"snapshots, installs — whose cost is a property of the machine. ",
		"<strong>Non-deterministic</strong> is a model deciding how long to ",
		"take. The slices are contiguous and sum to the wall clock, so ",
		"nothing hides in a residual.</p>")

	r.writeTimelineLegend()

	for _, fixture := range r.fixtures {
		r.writeFixtureTimeline(fixture)
	}

	r.write("</section>")
}

// writeTimelineLegend writes the color key for the gantt.
func (r *Renderer) writeTimelineLegend() {
	r.write(`<div class="legend">`,
		`<span class="sw" style="background:`, colorDeterministic, `"></span>deterministic`,
		`<span class="sw" style="background:`, colorPrompt, `"></span>prompt`,
		`<span class="sw" style="background:`, colorFix, `"></span>fix`,
		`<span class="sw" style="background:`, colorApply, `"></span>apply`,
		`<span class="sw" style="background:`, colorJudge, `"></span>judge`,
		"</div>")
}

// writeFixtureTimeline writes one fixture's phase table and gantt.
func (r *Renderer) writeFixtureTimeline(fixture *Fixture) {
	wall := timelineDenominator(fixture)

	r.write(`<div class="trace"><div class="trace-head">`,
		`<span class="name">`, escapeHTML(fixture.Name), `</span>`,
		`<span class="meta">`, formatDuration(fixture.Wall.Seconds(), fixture.HasWall),
		` wall · accounted `, formatSeconds(fixture.PhaseTotal),
		` · <a class="detail-link" href="`, escapeHTML(r.detailHref(fixture)),
		`">full detail →</a></span></div>`)

	r.write(`<table class="phases"><thead><tr><th>Phase</th><th>Kind</th>`,
		`<th class="num" title="time since the run began">Elapsed</th>`,
		`<th class="num">Duration</th><th class="num">% wall</th>`,
		"<th>Position in the run</th></tr></thead><tbody>")

	for _, phase := range fixture.Phases {
		r.writePhaseRow(phase, wall)
	}

	r.write("</tbody></table></div>")
}

// timelineDenominator is what the per-phase percentages are "of". It
// falls back to the accounted total when go test gave no wall clock, so
// a report built from engine logs alone still shows proportions.
func timelineDenominator(fixture *Fixture) float64 {
	if fixture.HasWall && fixture.Wall.Seconds() > 0 {
		return fixture.Wall.Seconds()
	}

	if fixture.PhaseTotal > 0 {
		return fixture.PhaseTotal
	}

	return 1
}

// writePhaseRow writes one slice: its label, kind, duration, share, and
// its position on the wall-clock axis.
func (r *Renderer) writePhaseRow(phase Phase, wall float64) {
	pct := shareOf(phase.Seconds, wall)

	note := ""
	if !phase.Measured {
		note = ` <span class="approx">residual</span>`
	}

	r.write(`<tr><td><div class="name">`, escapeHTML(phase.Label), note,
		`</div><div class="detail">`, escapeHTML(phase.Detail), `</div></td>`,
		"<td>", kindTag(phase.Kind), "</td>",
		`<td class="num">`, escapeHTML(formatElapsed(phase.Offset)), `</td>`,
		`<td class="num">`, formatSeconds(phase.Seconds), `</td>`,
		`<td class="num">`, formatPercent(pct, 1), `%</td>`,
		`<td class="wide">`,
		ganttBar(shareOf(phase.Offset, wall), pct, phaseColor(phase)),
		"</td></tr>")
}

// kindTag renders a slice's kind badge.
func kindTag(kind PhaseKind) string {
	switch kind {
	case KindDeterministic:
		return `<span class="tag det">deterministic</span>`
	case KindModel:
		return `<span class="tag mod">non-deterministic</span>`
	case KindMixed:
		return `<span class="tag mix">mixed</span>`
	default:
		return `<span class="tag det">` + escapeHTML(string(kind)) + `</span>`
	}
}
