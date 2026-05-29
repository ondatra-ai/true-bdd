package dto

// PlaywrightReport mirrors only the fields of the JSON reporter the
// runner needs. Future Playwright versions add fields; unknowns are
// ignored by encoding/json.
type PlaywrightReport struct {
	Suites []PlaywrightSuite `json:"suites"`
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
