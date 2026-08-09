package main

import "sort"

// ownerSummary is one row of the Go-side ownership table.
type ownerSummary struct {
	Owner   PhaseOwner
	Title   string
	Note    string
	Seconds float64
}

// ownerSummaries lists the three things that run Go-or-shell code during
// a fixture, in the order they first occur. Only the first is code
// true-bdd itself could make faster.
func ownerSummaries() []ownerSummary {
	return []ownerSummary{
		{
			Owner: OwnerEngine,
			Title: "Engine logic",
			Note: "config, checklist parse, template render, result " +
				"parsing between turns — true-bdd's own code",
		},
		{
			Owner: OwnerTests,
			Title: "Test subprocess",
			Note: "the framework runner the engine spawns (go test / " +
				"jest / npx playwright) — the project's tests, not " +
				"the engine",
		},
		{
			Owner: OwnerHarness,
			Title: "BDD harness",
			Note: "fixture scaffolding that exists only because this is " +
				"a test: tmpdir overlay, npm / playwright install, " +
				"snapshots, diff, teardown, judge",
		},
	}
}

// stepSummary is one row of the itemised deterministic table.
type stepSummary struct {
	Label   string
	Detail  string
	Owner   PhaseOwner
	Seconds float64
	Count   int
}

// writeGoSideBreakdown splits the deterministic total by who owns it,
// then itemises every slice.
func (r *Renderer) writeGoSideBreakdown() {
	r.write(`<section><h2>Where the Go-side time goes</h2>`,
		`<p class="lede">Not all deterministic time belongs to the engine. `,
		"Three different things run Go-or-shell code here, and only one of ",
		"them is code true-bdd could make faster.</p>")

	owners := r.ownerTotals()

	r.writeOwnerTiles(owners)
	r.writeOwnerTable(owners)

	r.write(`<p class="lede">Itemised, every deterministic slice — plus the `,
		"mixed post-run block, which the harness owns:</p>")
	r.writeStepTable()

	r.write("</section>")
}

// ownerTotals sums each owner's slices.
func (r *Renderer) ownerTotals() []ownerSummary {
	owners := ownerSummaries()

	for index := range owners {
		for _, fixture := range r.fixtures {
			for _, phase := range fixture.Phases {
				if phase.Owner == owners[index].Owner {
					owners[index].Seconds += phase.Seconds
				}
			}
		}
	}

	return owners
}

// writeOwnerTiles writes the three headline owner numbers.
func (r *Renderer) writeOwnerTiles(owners []ownerSummary) {
	// The harness's total is not purely Go: the post-run block it owns
	// is one measured span covering the snapshot, the diff, teardown AND
	// the judge's model call. Saying so on the tile keeps the number
	// from reading as pure Go time.
	mixed := r.mixedHarnessSeconds()

	r.write(`<div class="tiles">`)

	for _, owner := range owners {
		note := formatPercent(shareOf(owner.Seconds, r.totalWall), 1) + "% of wall clock"
		if owner.Owner == OwnerHarness && mixed > 0 {
			note = "incl. " + formatSeconds(mixed) +
				" post-run block that also contains the judge call"
		}

		r.write(`<div class="tile"><div class="k">`, escapeHTML(owner.Title),
			`</div><div class="v">`, formatSeconds(owner.Seconds),
			`</div><div class="n">`, escapeHTML(note), `</div></div>`)
	}

	r.write("</div>")
}

// mixedHarnessSeconds is the harness time that is not purely Go.
func (r *Renderer) mixedHarnessSeconds() float64 {
	total := 0.0

	for _, fixture := range r.fixtures {
		for _, phase := range fixture.Phases {
			if phase.Kind == KindMixed && phase.Owner == OwnerHarness {
				total += phase.Seconds
			}
		}
	}

	return total
}

// writeOwnerTable writes the owner rows, largest share first.
func (r *Renderer) writeOwnerTable(owners []ownerSummary) {
	total := 0.0
	for _, owner := range owners {
		total += owner.Seconds
	}

	if total == 0 {
		total = 1
	}

	ranked := make([]ownerSummary, len(owners))
	copy(ranked, owners)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Seconds > ranked[j].Seconds
	})

	r.write(`<table><thead><tr><th>Owner</th>`,
		`<th class="num">Total</th><th class="num">Share</th>`,
		"<th>Relative</th></tr></thead><tbody>")

	for _, owner := range ranked {
		share := shareOf(owner.Seconds, total)

		r.write(`<tr><td><div class="name">`, escapeHTML(owner.Title),
			`</div><div class="detail">`, escapeHTML(owner.Note), `</div></td>`,
			`<td class="num">`, formatSeconds(owner.Seconds), `</td>`,
			`<td class="num">`, formatPercent(share, 1), `%</td>`,
			`<td class="wide">`, bar(share, colorDeterministic, defaultBarHeight), "</td></tr>")
	}

	r.write("</tbody></table>")
}

// defaultBarHeight is the height of a summary bar in pixels.
const defaultBarHeight = 8

// writeStepTable itemises every non-model slice, largest first.
func (r *Renderer) writeStepTable() {
	steps := r.stepTotals()

	longest := 1.0
	for _, step := range steps {
		if step.Seconds > longest {
			longest = step.Seconds
		}
	}

	titles := map[PhaseOwner]string{}
	for _, owner := range ownerSummaries() {
		titles[owner.Owner] = owner.Title
	}

	r.write(`<table><thead><tr><th>Step</th><th>Owner</th>`,
		`<th class="num">Count</th><th class="num">Total</th>`,
		"<th>Relative</th></tr></thead><tbody>")

	for _, step := range steps {
		title, ok := titles[step.Owner]
		if !ok {
			title = string(step.Owner)
		}

		r.write(`<tr><td><div class="name">`, escapeHTML(step.Label),
			`</div><div class="detail">`, escapeHTML(step.Detail), `</div></td>`,
			`<td><span class="tag det">`, escapeHTML(title), `</span></td>`,
			`<td class="num">`, itoa(step.Count), `</td>`,
			`<td class="num">`, formatSeconds(step.Seconds), `</td>`,
			`<td class="wide">`,
			bar(shareOf(step.Seconds, longest), colorDeterministic, defaultBarHeight),
			"</td></tr>")
	}

	r.write("</tbody></table>")
}

// stepTotals groups every non-model slice by label, preserving first-seen
// order before ranking so equal totals stay in run order.
func (r *Renderer) stepTotals() []stepSummary {
	var steps []stepSummary

	index := map[string]int{}

	for _, fixture := range r.fixtures {
		for _, phase := range fixture.Phases {
			if phase.Kind == KindModel {
				continue
			}

			position, seen := index[phase.Label]
			if !seen {
				steps = append(steps, stepSummary{
					Label:  phase.Label,
					Detail: phase.Detail,
					Owner:  phase.Owner,
				})
				position = len(steps) - 1
				index[phase.Label] = position
			}

			steps[position].Seconds += phase.Seconds
			steps[position].Count++
		}
	}

	sort.SliceStable(steps, func(i, j int) bool {
		return steps[i].Seconds > steps[j].Seconds
	})

	return steps
}
