package main

import (
	"path/filepath"
	"strings"
)

// detailFileName is the page a fixture's row on the index links to.
func detailFileName(indexPath, fixtureName string) string {
	base := strings.TrimSuffix(filepath.Base(indexPath), filepath.Ext(indexPath))

	return base + "-" + fixtureName + ".html"
}

// DetailRenderer builds one fixture's page: the same timeline as the
// index, but every row opens into the evidence behind it.
type DetailRenderer struct {
	out     strings.Builder
	fixture *Fixture
	session string
	// indexHref links back to the summary.
	indexHref string
}

// NewDetailRenderer prepares a page for one fixture.
func NewDetailRenderer(fixture *Fixture, session, indexHref string) *DetailRenderer {
	return &DetailRenderer{fixture: fixture, session: session, indexHref: indexHref}
}

// Render produces the whole document.
func (d *DetailRenderer) Render() string {
	d.out.WriteString(documentHead)
	d.writeHeader()
	d.writeInvocation()
	d.writeTimeline()
	d.out.WriteString("</main></body>")

	return d.out.String()
}

// write appends raw markup.
func (d *DetailRenderer) write(parts ...string) {
	for _, part := range parts {
		d.out.WriteString(part)
	}
}

// writeHeader names the fixture and its verdict.
func (d *DetailRenderer) writeHeader() {
	fixture := d.fixture

	d.write("\n<header>\n  <h1>", escapeHTML(fixture.Name), "</h1>\n",
		`  <p class="sub"><a href="`, escapeHTML(d.indexHref), `">← run report</a> · session <code>`,
		escapeHTML(d.session), "</code> · ",
		formatDuration(fixture.Wall.Seconds(), fixture.HasWall), " wall · ",
		formatMoney(fixture.Cost+judgeCost(fixture)), " · ",
		verdictChip(fixture.Verdict), "</p>\n</header>\n")
}

// judgeCost is the fixture's judge spend, or zero when it never ran.
func judgeCost(fixture *Fixture) float64 {
	if fixture.Judge == nil {
		return 0
	}

	return fixture.Judge.CostUSD
}

// writeInvocation answers "what did the harness actually run?" before
// any timing — it is the frame every row below sits in.
func (d *DetailRenderer) writeInvocation() {
	fixture := d.fixture
	manifest := fixture.Manifest

	d.write(`<section><h2>How this run was started</h2>`)

	if manifest == nil || !manifest.Loaded {
		d.write(`<p class="lede">The fixture manifest could not be read — it may `,
			"have been renamed or removed since the run.</p></section>")

		return
	}

	d.writeCommandBlock("harness → CLI", manifest.Invocation(fixture.Dir))

	rows := [][2]string{
		{"working dir", fixture.Dir},
		{"checklist", metaOrDash(fixture.Meta.ChecklistPath)},
		{"architecture", metaOrDash(fixture.Meta.ArchitectureFile)},
		{"walk", walkSummary(fixture.Meta)},
	}

	if manifest.Answers != "" {
		rows = append(rows, [2]string{"stdin (answers)", manifest.Answers})
	}

	d.writeFactTable(rows)

	if fixture.Meta.TestRun.Known {
		d.writeCommandBlock("engine → test runner", fixture.Meta.TestRun)
	}

	d.write("</section>")
}

// walkSummary describes the shape of the checklist walk.
func walkSummary(meta RunMetadata) string {
	if meta.Items == 0 && meta.Prompts == 0 {
		return emDash
	}

	summary := itoa(meta.Items) + " item(s) × " + itoa(meta.Prompts) + " prompt(s)"
	if meta.MaxApplyAttempts > 0 {
		summary += " · max " + itoa(meta.MaxApplyAttempts) + " apply attempt(s)"
	}

	return summary
}

// metaOrDash renders an absent field as the report's absent marker.
func metaOrDash(value string) string {
	if value == "" {
		return emDash
	}

	return value
}

// writeTimeline writes the expandable phase list.
func (d *DetailRenderer) writeTimeline() {
	fixture := d.fixture

	d.write(`<section><h2>Timeline</h2>`,
		`<p class="lede">Every slice of the run, in order. Open a row for what `,
		"happened inside it — the command, the prompt, the response, the ",
		"tokens.</p>")

	wall := timelineDenominator(fixture)

	for index := range fixture.Phases {
		d.writePhaseDetail(fixture.Phases[index], wall)
	}

	d.write("</section>")
}

// writePhaseDetail writes one expandable row.
func (d *DetailRenderer) writePhaseDetail(phase Phase, wall float64) {
	pct := shareOf(phase.Seconds, wall)

	d.write(`<details class="row"><summary>`,
		`<span class="rk">`, kindTag(phase.Kind), `</span>`,
		`<span class="rl">`, escapeHTML(phase.Label), `</span>`,
		`<span class="rb">`, bar(pct, phaseColor(phase), defaultBarHeight), `</span>`,
		`<span class="rd">`, formatSeconds(phase.Seconds), `</span>`,
		`<span class="rp">`, formatPercent(pct, 1), `%</span>`,
		`</summary><div class="rbody">`)

	// A turn's phase detail is just "<cli> · <model>", which its own
	// fact table states properly a few lines further down. Every other
	// slice's detail is the only description it gets.
	if phase.Turn == nil {
		d.write(`<p class="detail">`, escapeHTML(phase.Detail), "</p>")
	}

	switch {
	case phase.Turn != nil:
		d.writeTurnDetail(phase.Turn)
	case phase.Label == "Fixture prep":
		d.writePrepDetail()
	case strings.HasPrefix(phase.Label, "Test run"):
		d.writeTestRunDetail()
	case phase.Label == "Checklist load + prompt render":
		d.writeChecklistDetail()
	case phase.Label == "Post-run + judge":
		d.writeJudgeDetail()
	default:
		d.writeEngineStepDetail()
	}

	d.write("</div></details>")
}

// writePrepDetail shows the scaffolding the harness laid down.
func (d *DetailRenderer) writePrepDetail() {
	manifest := d.fixture.Manifest
	if manifest == nil || !manifest.Loaded {
		d.writeNothingRecorded("the fixture manifest could not be read")

		return
	}

	d.writeFactTable([][2]string{
		{"repo layer (pre-copied)", strings.Join(repoLayerNames(), ", ")},
		{"input overlay", metaOrDash(manifest.InputPath)},
	})

	if len(manifest.PrepCmds) == 0 {
		d.write(`<p class="detail">No <code>prep:</code> commands — this slice is `,
			"the tmpdir overlay and the pre-run snapshot alone.</p>")

		return
	}

	d.writeCommandList("prep commands", manifest.PrepCmds)
}

// writeTestRunDetail shows the runner invocation and, for each
// subprocess it spawned, what the framework itself reported.
//
// The captured stdout is rendered expanded rather than summarized: for
// a suite that never started, the reason lives in that document's
// run-level errors[] block and nowhere else, so paraphrasing it is how
// the row ended up asserting "no tests ran" without being able to say
// why.
func (d *DetailRenderer) writeTestRunDetail() {
	fixture := d.fixture

	if fixture.Meta.TestRun.Known {
		d.writeCommandBlock("test runner", fixture.Meta.TestRun)
	} else {
		d.writeNothingRecorded("this run predates the engine logging its " +
			"test-runner command; the span is still measured from the log")
	}

	if len(fixture.TestRuns) == 0 {
		d.writeFactTable([][2]string{
			{"framework", metaOrDash(fixture.Discovery.Framework)},
			{"outcome", fixture.Discovery.Outcome},
		})
		d.writeNothingRecorded("this run predates the engine capturing the " +
			"runner's output; only the span between log records is known")

		return
	}

	for index := range fixture.TestRuns {
		d.writeTestRunOutcome(fixture.TestRuns[index])
	}
}

// writeTestRunOutcome renders one runner subprocess: its measurements,
// then both captured streams.
func (d *DetailRenderer) writeTestRunOutcome(run TestRun) {
	d.write(`<div class="fkey">`, escapeHTML(run.Label()), "</div>")

	d.writeFactTable([][2]string{
		{"outcome", run.Outcome()},
		{"duration", formatSeconds(run.Duration.Seconds())},
	})

	d.writeStream("stdout", run.Stdout, run.Framework+" wrote nothing to stdout")
	d.writeStream("stderr", run.Stderr, run.Framework+" wrote nothing to stderr")
}

// writeStream renders one captured stream. An empty-but-captured stream
// says so in its own words: it is a finding about the framework, not a
// gap in the report.
func (d *DetailRenderer) writeStream(name string, stream Artifact, emptyNote string) {
	switch {
	case stream.Path == "":
		return
	case stream.Missing:
		d.writeNothingRecorded(name + " was captured to " + stream.Path +
			", which is no longer on disk")
	case stream.Bytes == 0:
		d.write(`<div class="fkey">`, escapeHTML(name), ` <span class="sz">0 B</span></div>`,
			`<p class="detail none">`, escapeHTML(emptyNote),
			` — captured to <code>`, escapeHTML(stream.Path), "</code>.</p>")
	default:
		d.writeDocument(name+" — "+stream.Name, stream.Bytes, stream.Content)
	}
}

// writeChecklistDetail shows what was loaded and which cells will run.
func (d *DetailRenderer) writeChecklistDetail() {
	meta := d.fixture.Meta

	d.writeFactTable([][2]string{
		{"command", metaOrDash(meta.ChecklistCommand)},
		{"checklist file", metaOrDash(meta.ChecklistPath)},
		{"architecture", metaOrDash(meta.ArchitectureFile)},
		{"walk", walkSummary(meta)},
	})

	cells := d.fixture.cells()
	if len(cells) == 0 {
		return
	}

	d.write(`<div class="fkey">cells walked</div><ul class="plain">`)

	for _, cell := range cells {
		d.write("<li><code>", escapeHTML(cell), "</code></li>")
	}

	d.write("</ul>")
}

// cells lists the distinct checklist cells the run touched, in order.
func (f *Fixture) cells() []string {
	var (
		cells []string
		seen  = map[string]bool{}
	)

	for _, turn := range f.Turns {
		label := turn.Cell.String()
		if label == "" || seen[label] {
			continue
		}

		seen[label] = true

		cells = append(cells, label)
	}

	return cells
}

// writeEngineStepDetail shows what the engine wrote in a between-turns
// gap — the only visible product of a slice measured in milliseconds.
func (d *DetailRenderer) writeEngineStepDetail() {
	d.write(`<p class="detail">Pure Go: no subprocess, no model. The artifacts `,
		"this step produced are listed on the turns either side of it.</p>")
}

// writeJudgeDetail shows the harness's verdict machinery.
func (d *DetailRenderer) writeJudgeDetail() {
	fixture := d.fixture

	rows := [][2]string{{"judge model", cliClaude + " · sonnet"}}

	if fixture.Judge != nil {
		rows = append(rows,
			[2]string{"judge cost", formatMoney(fixture.Judge.CostUSD)},
			[2]string{"judge tokens", formatCount(fixture.Judge.Tokens)},
		)
	}

	rows = append(rows, [2]string{"verdict", metaOrDash(fixture.Verdict)})

	if fixture.Manifest != nil && fixture.Manifest.Loaded {
		rows = append(rows,
			[2]string{"expected exit", itoa(fixture.Manifest.ExpectedExit)},
			[2]string{"stdout checks", strings.Join(fixture.Manifest.StdoutChecks, " · ")},
		)
	}

	d.writeFactTable(rows)

	d.writeFailedChecks()

	if fixture.Manifest != nil && len(fixture.Manifest.TeardownCmds) > 0 {
		d.writeCommandList("teardown commands", fixture.Manifest.TeardownCmds)
	}

	if fixture.Manifest != nil && fixture.Manifest.JudgeSpec != "" {
		d.writeCollapsible("judge rubric", len(fixture.Manifest.JudgeSpec),
			fixture.Manifest.JudgeSpec)
	}

	d.writeFileDiff()
}

// writeFailedChecks lists the assertions that did not hold, including
// the judge's own reason — the one place the report says WHY a fixture
// failed rather than how long it took to.
func (d *DetailRenderer) writeFailedChecks() {
	if len(d.fixture.Failures) == 0 {
		return
	}

	d.write(`<div class="fkey">failed checks</div><ul class="plain">`)

	for _, failure := range d.fixture.Failures {
		d.write("<li>", escapeHTML(failure), "</li>")
	}

	d.write("</ul>")
}

// writeFileDiff lists what the run changed on disk. The harness prints
// this only when a fixture fails, so its absence on a pass is expected
// rather than missing data.
func (d *DetailRenderer) writeFileDiff() {
	fixture := d.fixture

	if len(fixture.Diff) == 0 {
		if fixture.Verdict == verdictPass {
			d.write(`<p class="detail">The file diff is printed by the harness `,
				"only for a failing fixture, so this passing run has none to show.</p>")
		}

		return
	}

	d.writeCollapsible("file diff · "+itoa(len(fixture.Diff))+" entries",
		0, strings.Join(fixture.Diff, "\n"))
}

// repoLayerNames is what the runner pre-copies into every tmpdir.
func repoLayerNames() []string {
	return []string{"true-bdd/", "templates/"}
}
