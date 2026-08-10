package reporter

import (
	"sort"
	"strings"
)

// traceBarHeight is the taller bar used in the per-turn trace.
const traceBarHeight = 10

// writeTurnInternals splits each turn into CLI boot, generation and
// teardown — the evidence that a model turn is not all model.
func (r *Renderer) writeTurnInternals() {
	r.write(`<section><h2>Inside the model turns</h2>`,
		`<p class="lede">A model turn is not all model. The claude provider `,
		"streams its session messages, so each turn splits into <strong>CLI ",
		"boot</strong> (process spawn, SessionStart hooks, MCP server and ",
		"plugin init — paid per turn because every turn is a fresh session), ",
		"<strong>generation</strong>, and result teardown. crush and codex ",
		"stream nothing, so their turns cannot be split.</p>")

	r.write(`<table><thead><tr><th>Turn</th><th>Runs on</th>`,
		`<th class="num">CLI boot</th><th class="num">Generation</th>`,
		`<th class="num">Teardown</th><th class="num">Total</th>`,
		"<th>Split</th></tr></thead><tbody>")

	for _, turn := range r.turns {
		r.writeTurnInternalRow(turn)
	}

	r.write("</tbody></table>")

	if r.bootTotal > 0 {
		r.write(`<p class="lede">Boot tax across the suite: <strong>`,
			formatSeconds(r.bootTotal), "</strong> spent starting CLI ",
			"sessions before any token was generated.</p>")
	}

	r.write("</section>")
}

// writeTurnInternalRow writes one turn's boot/generation/teardown split.
func (r *Renderer) writeTurnInternalRow(turn *Turn) {
	total := turn.Seconds()
	split := turn.Split()

	segments := `<div class="opaque">no stream — CLI is opaque</div>`
	if split.Known {
		segments = splitBar(split, total, turn.Role)
	}

	r.write(`<tr><td>#`, itoa(turn.Number), " ", roleDot(turn.Role), `</td>`,
		`<td><code>`, escapeHTML(turn.CLI), `</code></td>`,
		`<td class="num">`, formatDuration(split.Boot.Seconds(), split.Known), `</td>`,
		`<td class="num">`, formatDuration(split.Generation.Seconds(), split.Known), `</td>`,
		`<td class="num">`, formatDuration(split.Teardown.Seconds(), split.Known), `</td>`,
		`<td class="num">`, formatSeconds(total), `</td>`,
		`<td class="wide">`, segments, "</td></tr>")
}

// splitBar renders the three-segment bar for a splittable turn.
func splitBar(split BootSplit, total float64, role string) string {
	segments := []struct {
		seconds float64
		color   string
	}{
		{split.Boot.Seconds(), colorBoot},
		{split.Generation.Seconds(), roleColor(role)},
		{split.Teardown.Seconds(), colorTeardown},
	}

	var out strings.Builder

	out.WriteString(`<div class="split">`)

	for _, segment := range segments {
		out.WriteString(`<span style="width:` +
			formatPercent(shareOf(segment.seconds, total), percentDecimals) +
			`%;background:` + segment.color + `"></span>`)
	}

	out.WriteString("</div>")

	return out.String()
}

// roleTotals is one row of the by-role table.
type roleTotals struct {
	Role    string
	Turns   int
	Seconds float64
	Cost    float64
	Tokens  int
	CLIs    map[string]bool
}

// writeRoleTable writes what each checklist role cost in time and money.
func (r *Renderer) writeRoleTable() {
	r.write(`<section><h2>Cost and time by role</h2>`)
	r.write(`<table><thead><tr><th>Role</th><th>Runs on</th>`,
		`<th class="num">Turns</th><th class="num">Time</th>`,
		`<th class="num">Tokens</th><th class="num">Cost</th>`,
		"<th>Share of model time</th></tr></thead><tbody>")

	for _, entry := range r.roleTotals() {
		clis := sortedKeys(entry.CLIs)

		timeCell := emDash
		if entry.Seconds > 0 {
			timeCell = formatSeconds(entry.Seconds)
		}

		r.write("<tr><td>", roleDot(entry.Role), "</td>",
			`<td><code>`, escapeHTML(strings.Join(clis, ", ")), `</code></td>`,
			`<td class="num">`, itoa(entry.Turns), `</td>`,
			`<td class="num">`, timeCell, `</td>`,
			`<td class="num">`, formatCount(entry.Tokens), `</td>`,
			`<td class="num">`, formatMoney(entry.Cost), `</td>`,
			`<td class="wide">`,
			bar(shareOf(entry.Seconds, r.modelTime), roleColor(entry.Role), defaultBarHeight),
			"</td></tr>")
	}

	r.write("</tbody></table>")
	r.write(`<p class="lede">The judge has no duration of its own here: it runs `,
		"inside the post-run block, whose measured span covers the snapshot, ",
		"the diff, teardown and the call together.</p>")
	r.write("</section>")
}

// roleTotals groups turns by role, in engine-run order, with the judge
// folded in from the harness's own records.
func (r *Renderer) roleTotals() []roleTotals {
	byRole := map[string]*roleTotals{}

	var order []string

	ensure := func(role string) *roleTotals {
		entry, ok := byRole[role]
		if !ok {
			entry = &roleTotals{Role: role, CLIs: map[string]bool{}}
			byRole[role] = entry
			order = append(order, role)
		}

		return entry
	}

	for _, turn := range r.turns {
		role := turn.Role
		if role == "" {
			role = emDash
		}

		entry := ensure(role)
		entry.Turns++
		entry.Seconds += turn.Seconds()
		entry.Cost += turn.CostUSD
		entry.Tokens += turn.Tokens.Total()

		if turn.CLI != "" {
			entry.CLIs[turn.CLI] = true
		}
	}

	for _, fixture := range r.fixtures {
		if fixture.Judge == nil {
			continue
		}

		entry := ensure(roleJudge)
		entry.CLIs[cliClaude] = true
		entry.Turns++
		entry.Cost += fixture.Judge.CostUSD
		entry.Tokens += fixture.Judge.Tokens
	}

	return rankRoles(byRole, order)
}

// rankRoles puts the known roles in engine-run order and appends any
// unrecognised ones in the order they were seen.
func rankRoles(byRole map[string]*roleTotals, order []string) []roleTotals {
	var out []roleTotals

	seen := map[string]bool{}

	for _, role := range roleOrder() {
		if entry, ok := byRole[role]; ok {
			out = append(out, *entry)
			seen[role] = true
		}
	}

	for _, role := range order {
		if !seen[role] {
			out = append(out, *byRole[role])
		}
	}

	return out
}

// cliTotals is one row of the by-CLI table.
type cliTotals struct {
	CLI      string
	Model    string
	Turns    int
	Seconds  float64
	Cost     float64
	Tokens   int
	Reported int
}

// writeCLITable writes per-CLI totals, and how much of it each CLI
// actually reported.
func (r *Renderer) writeCLITable() {
	r.write(`<section><h2>By CLI and model</h2>`)
	r.write(`<table><thead><tr><th>CLI · model</th>`,
		`<th class="num">Turns</th><th class="num">Time</th>`,
		`<th class="num">Tokens</th><th class="num">Cost</th>`,
		"<th>Usage reported</th></tr></thead><tbody>")

	for _, entry := range r.cliTotals() {
		r.write(`<tr><td><code>`, escapeHTML(entry.CLI), `</code> · <code>`,
			escapeHTML(entry.Model), `</code></td>`,
			`<td class="num">`, itoa(entry.Turns), `</td>`,
			`<td class="num">`, formatSeconds(entry.Seconds), `</td>`,
			`<td class="num">`, formatCount(entry.Tokens), `</td>`,
			`<td class="num">`, formatMoney(entry.Cost), `</td>`,
			"<td>", reportedCell(entry), "</td></tr>")
	}

	r.write("</tbody></table></section>")
}

// reportedCell says how many of a CLI's turns came with usage data.
// A CLI that reports nothing is called out: its spend is real but
// invisible everywhere else in this report.
func reportedCell(entry cliTotals) string {
	if entry.Reported == 0 {
		return `<span class="bad">none — CLI reports no usage</span>`
	}

	if entry.Reported < entry.Turns {
		return `<span class="warn">` + itoa(entry.Reported) + "/" +
			itoa(entry.Turns) + " turns</span>"
	}

	return itoa(entry.Reported) + "/" + itoa(entry.Turns) + " turns"
}

// cliTotals groups turns by (cli, model), sorted by name.
func (r *Renderer) cliTotals() []cliTotals {
	byCLI := map[string]*cliTotals{}

	for _, turn := range r.turns {
		key := turn.CLI + "\x00" + turn.Model

		entry, ok := byCLI[key]
		if !ok {
			entry = &cliTotals{CLI: turn.CLI, Model: turn.Model}
			byCLI[key] = entry
		}

		entry.Turns++
		entry.Seconds += turn.Seconds()
		entry.Cost += turn.CostUSD
		entry.Tokens += turn.Tokens.Total()

		if turn.HasCost || turn.Tokens.Total() > 0 {
			entry.Reported++
		}
	}

	out := make([]cliTotals, 0, len(byCLI))
	for _, key := range sortedKeys(byCLI) {
		out = append(out, *byCLI[key])
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CLI != out[j].CLI {
			return out[i].CLI < out[j].CLI
		}

		return out[i].Model < out[j].Model
	})

	return out
}

// writeTurnTrace writes every call the engine made, in order.
func (r *Renderer) writeTurnTrace() {
	r.write(`<section><h2>Turn-by-turn trace</h2>`)

	for _, fixture := range r.fixtures {
		r.write(`<div class="trace"><div class="trace-head">`,
			`<span class="name">`, escapeHTML(fixture.Name), `</span>`,
			`<span class="meta">`, itoa(len(fixture.Turns)), ` turns · `,
			formatSeconds(fixture.ModelSeconds), ` model time · `,
			formatMoney(fixture.Cost), `</span></div>`)

		if len(fixture.Turns) == 0 {
			r.write(`<div class="empty">no AI turns recorded</div>`)
		}

		for _, turn := range fixture.Turns {
			r.writeTraceRow(turn)
		}

		r.write("</div>")
	}

	r.write("</section>")
}

// writeTraceRow writes one turn's line in the trace.
func (r *Renderer) writeTraceRow(turn *Turn) {
	seconds := turn.Seconds()

	flag := ""

	switch turn.Status {
	case TurnOpen:
		flag = `<span class="bad">no completion record</span>`
	case TurnFailed:
		flag = `<span class="bad">failed</span>`
	case TurnOK:
	}

	floor := ""
	if turn.DurationIsFloor {
		floor = "≥ "
	}

	r.write(`<div class="turn">`,
		`<span class="tn">#`, itoa(turn.Number), `</span>`,
		`<span class="tr">`, roleDot(turn.Role), `</span>`,
		`<span class="tc"><code>`, escapeHTML(turn.CLI), `</code></span>`,
		`<span class="tb">`,
		bar(shareOf(seconds, r.maxTurn), roleColor(turn.Role), traceBarHeight),
		`</span>`,
		`<span class="td">`, floor, formatSeconds(seconds), `</span>`,
		`<span class="tk">`, formatCount(turn.Tokens.Total()), `</span>`,
		`<span class="tp">`, formatMoney(turn.CostUSD), `</span>`,
		`<span class="tf">`, flag, `</span>`,
		"</div>")
}
