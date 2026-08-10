package reporter

import "strings"

// finding is one conclusion the timeline itself supports.
type finding struct {
	Title string
	Body  string
}

// writeFindings writes the conclusions the data supports — and nothing
// else. An uneventful run gets no section at all rather than a
// reassuring empty one.
func (r *Renderer) writeFindings() {
	findings := r.collectFindings()
	if len(findings) == 0 {
		return
	}

	r.write(`<section><h2>What the timeline says</h2>`, `<div class="findings">`)

	for _, item := range findings {
		r.write(`<div class="finding"><div class="ft">`, item.Title,
			`</div><div class="fb">`, item.Body, `</div></div>`)
	}

	r.write("</div></section>")
}

// collectFindings gathers every supported finding, in report order.
func (r *Renderer) collectFindings() []finding {
	var findings []finding

	for _, fixture := range r.fixtures {
		if item, ok := suiteNeverRanFinding(fixture); ok {
			findings = append(findings, item)
		}

		if item, ok := emptyFailureFinding(fixture); ok {
			findings = append(findings, item)
		}
	}

	if item, ok := r.bootTaxFinding(); ok {
		findings = append(findings, item)
	}

	return findings
}

// suiteNeverRanFinding fires when the engine fell back to its startup
// marker: the runner reported no test results at all, so everything
// downstream was driven by a placeholder rather than a real failure.
func suiteNeverRanFinding(fixture *Fixture) (finding, bool) {
	if !strings.Contains(fixture.Discovery.Outcome, "never started") {
		return finding{}, false
	}

	span := 0.0
	known := false

	if !fixture.Discovery.End.IsZero() && !fixture.First.IsZero() {
		span = fixture.Discovery.End.Sub(fixture.First).Seconds()
		known = true
	}

	return finding{
		Title: "The suite never ran — zero tests executed",
		Body: "<code>" + escapeHTML(fixture.Name) + "</code>'s " +
			formatDuration(span, known) + " " + escapeHTML(fixture.Discovery.Framework) +
			" slice is a <strong>startup failure</strong>, not a test run: the " +
			"engine fell back to its <code>&lt;startup&gt;</code> marker subject, " +
			"which it only emits when the runner exits non-zero having reported " +
			"no test results at all. Everything downstream — every turn, every " +
			"dollar — is driven by that placeholder, not by a real failing test.",
	}, true
}

// emptyFailureFinding fires when a prompt described a failing test but
// carried no failure text for the model to work from.
func emptyFailureFinding(fixture *Fixture) (finding, bool) {
	if len(fixture.EmptyFailurePrompts) == 0 {
		return finding{}, false
	}

	return finding{
		Title: "The fix turn was handed an empty failure",
		Body: itoa(len(fixture.EmptyFailurePrompts)) + " prompt(s) in <code>" +
			escapeHTML(fixture.Name) + "</code> contain a <em>Last Failure " +
			"Output</em> block with nothing in it. The runner's startup " +
			"subject fills that field from the child process's " +
			"<strong>stderr</strong>, but Playwright reports a webServer startup " +
			"failure in its JSON report's top-level <code>errors[]</code> on " +
			"stdout — which the parsed report struct does not keep. The model had " +
			"to infer the failure it was asked to fix.",
	}, true
}

// bootTaxFinding fires when more time went to starting CLI sessions than
// to all the deterministic work in the run.
func (r *Renderer) bootTaxFinding() (finding, bool) {
	if r.bootTotal <= r.deterministic {
		return finding{}, false
	}

	return finding{
		Title: "CLI boot outweighs all engine logic",
		Body: formatSeconds(r.bootTotal) + " went to starting CLI sessions " +
			"before a single token was generated — more than the " +
			formatSeconds(r.deterministic) + " of deterministic work in the whole " +
			"run. Every turn opens a fresh session, so the boot is paid per turn, " +
			"not per run.",
	}, true
}
