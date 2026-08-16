package reportserver

import "sort"

// MatrixRun is one column: a run, identified and sized.
type MatrixRun struct {
	ID string `json:"id"`
	// Label is the run's start time as a reader says it out loud,
	// "2026-08-10 23:02:40", rather than the directory's underscored form.
	Label string `json:"label"`
	// Mode is the column's AI-CLI mode, "" when unknown. A replayed
	// column proves the engine still behaves as recorded; a live one
	// proves the models still do. Reading the grid without it compares
	// two different claims.
	Mode     string `json:"mode"`
	Fixtures int    `json:"fixtures"`
	Complete bool   `json:"complete"`
	// Partial marks a session that ran only a fraction of the suite —
	// almost always a targeted re-run of whatever was being debugged.
	// Flagged because such a column is not evidence about the whole suite.
	Partial bool `json:"partial"`
}

// MatrixCell is one test in one run.
type MatrixCell struct {
	Run     string `json:"run"`
	Outcome string `json:"outcome"`
	// Label is the full sentence, for the tooltip; Reason is the short
	// form written in the cell. Failed drives the colour: red or green,
	// nothing else — the words carry the distinctions.
	Label       string  `json:"label"`
	Reason      string  `json:"reason"`
	Failed      bool    `json:"failed"`
	WallSeconds float64 `json:"wall_seconds"`
	HasWall     bool    `json:"has_wall"`
	ExitCode    *int    `json:"exit_code"`
	// Detail is the first failure line, for the cell's tooltip.
	Detail string `json:"detail"`
}

// MatrixRow is one test across every run. Cells are index-aligned to
// MatrixResponse.Runs, so the client indexes rather than looks up.
type MatrixRow struct {
	Name     string         `json:"name"`
	Cells    []MatrixCell   `json:"cells"`
	Observed int            `json:"observed"`
	Passed   int            `json:"passed"`
	Failed   int            `json:"failed"`
	Reasons  map[string]int `json:"reasons"`
}

// MatrixResponse is the whole grid.
type MatrixResponse struct {
	Version int64       `json:"version"`
	Runs    []MatrixRun `json:"runs"`
	Tests   []MatrixRow `json:"tests"`
}

// partialThreshold is the fraction of the largest run's fixture count
// below which a session counts as a targeted re-run rather than a full
// suite execution.
const partialThreshold = 0.8

// sessionNameLength is the width of a session directory name,
// "2006-01-02_15-04-05".
const sessionNameLength = len("2006-01-02_15-04-05")

// BuildMatrix projects every run and every test onto one grid.
//
// Columns run oldest to newest so the eye reads history left to right —
// the opposite of the run list, which leads with the newest because that
// is the one you came to look at.
func BuildMatrix(snapshot *Snapshot) MatrixResponse {
	response := MatrixResponse{
		Version: snapshot.Version,
		Runs:    matrixRuns(snapshot),
		Tests:   []MatrixRow{},
	}

	for _, name := range testNames(snapshot) {
		response.Tests = append(response.Tests, buildRow(name, snapshot))
	}

	return response
}

// matrixRuns builds the column headers, flagging the partial sessions.
func matrixRuns(snapshot *Snapshot) []MatrixRun {
	largest := 0
	for _, run := range snapshot.Runs {
		if run.Totals.Fixtures > largest {
			largest = run.Totals.Fixtures
		}
	}

	runs := make([]MatrixRun, 0, len(snapshot.Runs))

	for _, run := range snapshot.Runs {
		runs = append(runs, MatrixRun{
			ID:       run.ID,
			Label:    runLabel(run.ID),
			Mode:     run.Mode,
			Fixtures: run.Totals.Fixtures,
			Complete: run.Complete,
			Partial:  largest > 0 && float64(run.Totals.Fixtures) < float64(largest)*partialThreshold,
		})
	}

	return runs
}

// runLabel turns a session directory name into a readable timestamp:
// "2026-08-10_23-02-40" becomes "2026-08-10 23:02:40".
//
// Formatted here rather than in the browser so every surface that shows
// a run says it the same way.
func runLabel(id string) string {
	if len(id) != sessionNameLength {
		return id
	}

	date, clock := id[:10], id[11:]

	return date + " " + clock[0:2] + ":" + clock[3:5] + ":" + clock[6:8]
}

// testNames is every test seen in any run, sorted.
//
// The union rather than the newest run's list: a fixture that was
// deleted, renamed, or only ever ran in older sessions still has history
// worth showing, and dropping its row would silently rewrite the past.
func testNames(snapshot *Snapshot) []string {
	seen := map[string]struct{}{}

	for _, run := range snapshot.Runs {
		for _, fixture := range run.Tests {
			seen[fixture.Name] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// buildRow assembles one test's cells and counts.
func buildRow(name string, snapshot *Snapshot) MatrixRow {
	row := MatrixRow{
		Name:    name,
		Cells:   make([]MatrixCell, 0, len(snapshot.Runs)),
		Reasons: map[string]int{},
	}

	for _, run := range snapshot.Runs {
		cell := buildCell(run, name)
		row.Cells = append(row.Cells, cell)

		outcome := Outcome(cell.Outcome)
		if !outcome.Observed() {
			continue
		}

		row.Observed++

		if outcome.Failed() {
			row.Failed++
			row.Reasons[cell.Outcome]++

			continue
		}

		row.Passed++
	}

	return row
}

// buildCell resolves one test in one run.
func buildCell(run *Run, name string) MatrixCell {
	fixture, ok := run.Test(name)
	if !ok {
		return MatrixCell{
			Run:     run.ID,
			Outcome: string(OutcomeAbsent),
			Label:   OutcomeAbsent.Label(),
		}
	}

	summary := mapTestSummary(fixture)
	outcome := classify(summary)

	detail := ""
	if len(summary.Failures) > 0 {
		detail = summary.Failures[0]
	}

	return MatrixCell{
		Run:         run.ID,
		Outcome:     string(outcome),
		Label:       outcome.Label(),
		Reason:      outcome.Reason(),
		Failed:      outcome.Failed(),
		WallSeconds: summary.WallSeconds,
		HasWall:     summary.HasWall,
		ExitCode:    summary.ExitCode,
		Detail:      detail,
	}
}
