package reporter

import "strings"

// writeTurnDetail is the heart of the detail page: what the engine sent,
// what it permitted, what came back, and what it cost.
func (d *DetailRenderer) writeTurnDetail(turn *Turn) {
	d.writeTurnFacts(turn)
	d.writeTurnInvocation(turn)
	d.writeTurnTokens(turn)
	d.writeTurnArtifacts(turn)
	d.writeTurnToolCalls(turn)
}

// writeTurnFacts writes the turn's identity: which cell, which model,
// how the time inside it divided.
func (d *DetailRenderer) writeTurnFacts(turn *Turn) {
	split := turn.Split()

	rows := [][2]string{
		{"checklist cell", cellOrDash(turn.Cell)},
		{"role", turn.Role},
		{"runs on", turn.CLI + " · " + turn.Model},
	}

	if split.Known {
		rows = append(rows, [2]string{"time split",
			"CLI boot " + formatSeconds(split.Boot.Seconds()) +
				" · generation " + formatSeconds(split.Generation.Seconds()) +
				" · teardown " + formatSeconds(split.Teardown.Seconds())})
	} else {
		rows = append(rows, [2]string{"time split",
			"not splittable — " + turn.CLI + " streams no session messages"})
	}

	if turn.Status == TurnFailed && turn.Error != "" {
		rows = append(rows, [2]string{"error", turn.Error})
	}

	if turn.Status == TurnOpen {
		rows = append(rows, [2]string{"status",
			"no completion record — the process stopped writing mid-turn"})
	}

	d.writeFactTable(rows)
}

// cellOrDash renders the checklist cell, or the absent marker.
func cellOrDash(cell checklistCell) string {
	if label := cell.String(); label != "" {
		return label
	}

	return emDash
}

// writeTurnInvocation shows the command and the permissions it ran
// under — the two things that decide what a turn was even able to do.
func (d *DetailRenderer) writeTurnInvocation(turn *Turn) {
	d.writeCommandBlock("invocation", resolveInvocation(turn))

	var rows [][2]string

	if len(turn.AllowedTools) > 0 {
		rows = append(rows, [2]string{"allowed tools", strings.Join(turn.AllowedTools, ", ")})
	}

	if len(turn.DisallowedTools) > 0 {
		rows = append(rows, [2]string{"denied tools", strings.Join(turn.DisallowedTools, ", ")})
	}

	if len(rows) == 0 {
		return
	}

	d.writeFactTable(rows)
}

// writeTurnTokens breaks usage out by counter. The four are priced
// differently, so the sum alone cannot say whether a turn was expensive
// because it thought hard or because it re-uploaded its system prompt.
func (d *DetailRenderer) writeTurnTokens(turn *Turn) {
	if !turn.HasCost && turn.Tokens.Total() == 0 {
		d.writeNothingRecorded(turn.CLI + " reports no usage, so this turn's " +
			"real spend is invisible to every cost column in this report")

		return
	}

	tokens := turn.Tokens

	d.writeFactTable([][2]string{
		{"input", formatCount(tokens.Input)},
		{"output", formatCount(tokens.Output)},
		{"cache read", formatCount(tokens.CacheRead)},
		{"cache write", formatCount(tokens.CacheCreation)},
		{"total", formatCount(tokens.Total())},
		{"cost", formatMoney(turn.CostUSD)},
	})
}

// writeTurnArtifacts embeds the prompts and responses verbatim. They are
// the point of the page: a timing without the prompt that produced it
// explains nothing.
func (d *DetailRenderer) writeTurnArtifacts(turn *Turn) {
	artifacts := make([]Artifact, 0, len(turn.Inputs)+len(turn.Outputs))
	artifacts = append(artifacts, turn.Inputs...)
	artifacts = append(artifacts, turn.Outputs...)

	if len(artifacts) == 0 {
		d.writeNothingRecorded("no prompt artifacts recorded for this turn")

		return
	}

	for _, artifact := range artifacts {
		if artifact.Missing {
			d.write(`<div class="fkey">`, escapeHTML(artifact.Kind),
				`</div><p class="detail">`, escapeHTML(artifact.Name),
				" — written by the run, no longer on disk</p>")

			continue
		}

		d.writeCollapsible(artifact.Kind+" · "+artifact.Name,
			artifact.Bytes, artifact.Content)
	}
}

// writeTurnToolCalls lists what the model actually did. A turn whose
// prompt and permissions look right but which made no calls at all is a
// different failure from one that searched and found nothing.
func (d *DetailRenderer) writeTurnToolCalls(turn *Turn) {
	if len(turn.ToolCalls) == 0 {
		return
	}

	d.write(`<div class="fkey">tool calls · `,
		escapeHTML(summariseToolCalls(turn.ToolCalls)), `</div>`,
		`<table class="tools"><tbody>`)

	for index, call := range turn.ToolCalls {
		d.write(`<tr><td class="num">`, itoa(index+1), `</td>`,
			`<td><code>`, escapeHTML(call.Name), `</code></td>`,
			`<td><code class="wrap">`, escapeHTML(call.Input), `</code></td></tr>`)
	}

	d.write("</tbody></table>")
}
