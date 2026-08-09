package main

import "strings"

// writeFactTable writes a two-column key/value block. Empty values are
// skipped rather than rendered blank, so a row's absence means "not
// recorded" instead of "recorded as nothing".
func (d *DetailRenderer) writeFactTable(rows [][2]string) {
	kept := make([][2]string, 0, len(rows))

	for _, row := range rows {
		if strings.TrimSpace(row[1]) != "" {
			kept = append(kept, row)
		}
	}

	if len(kept) == 0 {
		return
	}

	d.write(`<table class="facts"><tbody>`)

	for _, row := range kept {
		d.write(`<tr><td class="fk">`, escapeHTML(row[0]), `</td>`,
			`<td class="fv">`, factValue(row[1]), "</td></tr>")
	}

	d.write("</tbody></table>")
}

// factValue renders a value, keeping a multi-line one readable.
func factValue(value string) string {
	if strings.Contains(value, "\n") {
		return `<pre class="inline">` + escapeHTML(value) + "</pre>"
	}

	return escapeHTML(value)
}

// writeCommandBlock writes one copyable command line, flagged when it is
// a reconstruction rather than a record.
func (d *DetailRenderer) writeCommandBlock(label string, invocation Invocation) {
	if !invocation.Known {
		d.writeNothingRecorded("no command recorded for " + label)

		return
	}

	suffix := ""
	if invocation.Reconstructed {
		suffix = ` <span class="approx">reconstructed from logged options</span>`
	}

	d.write(`<div class="fkey">`, escapeHTML(label), suffix, `</div>`,
		`<pre class="cmd">`, escapeHTML(invocation.Command()), "</pre>")

	if invocation.Dir != "" {
		d.write(`<p class="detail">in <code>`, escapeHTML(invocation.Dir), "</code></p>")
	}
}

// writeCommandList writes shell commands verbatim, in order.
func (d *DetailRenderer) writeCommandList(label string, commands []string) {
	if len(commands) == 0 {
		return
	}

	d.write(`<div class="fkey">`, escapeHTML(label), `</div><pre class="cmd">`)

	for index, command := range commands {
		if index > 0 {
			d.write("\n")
		}

		d.write(escapeHTML(command))
	}

	d.write("</pre>")
}

// writeCollapsible embeds a full document behind a summary line. Nested
// inside an already-open row, so a page of twelve turns is navigable
// without hiding anything.
func (d *DetailRenderer) writeCollapsible(label string, size int, content string) {
	sizeChip := ""
	if size > 0 {
		sizeChip = ` <span class="sz">` + formatBytes(size) + `</span>`
	}

	d.write(`<details class="doc"><summary>`, escapeHTML(label), sizeChip,
		"</summary><pre>", escapeHTML(content), "</pre></details>")
}

// writeNothingRecorded states an absence explicitly. An expander that
// simply renders empty reads as a bug in the report; one that says why
// reads as a fact about the run.
func (d *DetailRenderer) writeNothingRecorded(reason string) {
	d.write(`<p class="detail none">Not recorded — `, escapeHTML(reason), ".</p>")
}

// bytesPerKiB is the divisor for the human-readable artifact sizes.
const bytesPerKiB = 1024

// formatBytes renders an artifact size.
func formatBytes(size int) string {
	if size < bytesPerKiB {
		return itoa(size) + " B"
	}

	return formatPercent(float64(size)/bytesPerKiB, 1) + " KB"
}
