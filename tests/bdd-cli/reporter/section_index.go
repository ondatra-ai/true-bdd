package reporter

// The index answers one question — what ran, and did it pass. Anything
// that needs interpreting to be useful (where a fixture's wall clock
// went, what each turn cost, which slices are engine versus model)
// lives on the fixture's own page, where the reader has asked for it.
//
// Every number here is read straight out of a log or a harness record:
// the verdict and wall clock from harness.json, the turn count and cost
// from the engine's own `Dispatching AI turn` / `AI turn usage`
// records. Nothing is derived from a model of what the harness was
// doing between those records.

// writeFixtureList writes the report's body: one row per fixture, its
// verdict, and how long it took.
func (r *Renderer) writeFixtureList() {
	r.write(`<section><table><thead><tr>`,
		`<th>Fixture</th><th>Status</th><th class="num">Time</th>`,
		`</tr></thead><tbody>`)

	for _, fixture := range r.fixtures {
		r.write(`<tr><td><a href="`, escapeHTML(r.detailHref(fixture)), `">`,
			escapeHTML(fixture.Name), `</a></td>`,
			`<td>`, verdictChip(fixture.Verdict), `</td>`,
			`<td class="num">`, escapeHTML(fixtureWall(fixture)), `</td></tr>`)
	}

	r.write(`</tbody></table></section>`)
}

// writeRunSummary writes the totals, after the list rather than before
// it: the per-fixture outcomes are the point of the page, and the
// aggregate is what you check once you have read them.
func (r *Renderer) writeRunSummary() {
	total := len(r.fixtures)
	failed := total - r.passed

	wall := emDash
	if r.totalWall > 0 {
		wall = formatSeconds(r.totalWall)
	}

	tiles := [][2]string{
		{"Fixtures", itoa(total)},
		{"Passed", itoa(r.passed)},
		{"Failed", itoa(failed)},
		{"Wall clock", wall},
		{"AI turns", itoa(len(r.turns))},
		{"Cost", formatMoney(r.engineCost + r.judgeCost)},
	}

	r.write(`<section><h2>Summary</h2><div class="tiles">`)

	for _, tile := range tiles {
		r.write(`<div class="tile"><div class="k">`, escapeHTML(tile[0]),
			`</div><div class="v">`, escapeHTML(tile[1]), `</div></div>`)
	}

	r.write("</div></section>")
}

// fixtureWall is a fixture's measured wall clock, or an em dash when
// the harness recorded none (a fixture that never reached its cleanup).
func fixtureWall(fixture *Fixture) string {
	if !fixture.HasWall {
		return emDash
	}

	return formatSeconds(fixture.Wall.Seconds())
}
