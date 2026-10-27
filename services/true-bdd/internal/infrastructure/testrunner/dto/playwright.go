package dto

// PlaywrightReport mirrors only the fields of the JSON reporter the
// runner needs. Future Playwright versions add fields; unknowns are
// ignored by encoding/json.
type PlaywrightReport struct {
	Suites []PlaywrightSuite `json:"suites"`
	// Errors holds run-level failures that belong to no individual test —
	// a webServer that failed to boot, a config that wouldn't load.
	// Playwright writes these to the report on stdout, never stderr.
	Errors []PlaywrightError `json:"errors,omitempty"`
	Stats  PlaywrightStats   `json:"stats"`
}

// PlaywrightStats is the run-level summary block. Playwright emits it
// even when zero tests executed, which makes StartTime the field that
// separates "the suite ran" from "the process died before it started".
type PlaywrightStats struct {
	StartTime string `json:"startTime"`
	// Duration is the suite's own wall-clock measurement in
	// milliseconds, fractional as Playwright writes it.
	Duration   float64 `json:"duration"`
	Expected   int     `json:"expected"`
	Unexpected int     `json:"unexpected"`
	Skipped    int     `json:"skipped"`
	Flaky      int     `json:"flaky"`
}

// PlaywrightSuite is one entry in the JSON `suites:` tree. May contain
// nested suites (describe blocks) and/or specs (leaf tests).
type PlaywrightSuite struct {
	Title  string            `json:"title"`
	File   string            `json:"file"`
	Specs  []PlaywrightSpec  `json:"specs"`
	Suites []PlaywrightSuite `json:"suites"`
}

// PlaywrightSpec is one `test(...)` callsite, possibly run across
// multiple Playwright projects (entries in tests[]).
type PlaywrightSpec struct {
	Title string           `json:"title"`
	File  string           `json:"file"`
	Tests []PlaywrightTest `json:"tests"`
}

// PlaywrightTest is one project's execution of a spec. Results[] holds
// the per-attempt outcomes (Playwright retries are appended).
type PlaywrightTest struct {
	Results []PlaywrightResult `json:"results"`
}

// PlaywrightResult is one attempt's verdict plus the error block when
// it failed.
type PlaywrightResult struct {
	Status string             `json:"status"`
	Errors []PlaywrightError  `json:"errors,omitempty"`
	Stdout []PlaywrightOutput `json:"stdout,omitempty"`
	Stderr []PlaywrightOutput `json:"stderr,omitempty"`
}

// PlaywrightError carries the human-readable failure message from one
// failed result.
type PlaywrightError struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

// PlaywrightOutput is one stdout/stderr chunk attached to a result.
type PlaywrightOutput struct {
	Text string `json:"text"`
}
