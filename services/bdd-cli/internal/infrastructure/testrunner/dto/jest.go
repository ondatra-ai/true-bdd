package dto

// JestReport mirrors the fields of `npx jest --json` output the runner
// needs. The camelCase JSON keys match jest's wire contract.
type JestReport struct {
	TestResults []JestTestResult `json:"testResults"`
}

// JestTestResult is one spec file's worth of execution.
type JestTestResult struct {
	Name             string                `json:"name"`
	AssertionResults []JestAssertionResult `json:"assertionResults"`
}

// JestAssertionResult is one `test(...)` or `it(...)` callsite's
// outcome.
type JestAssertionResult struct {
	FullName        string   `json:"fullName"`
	Status          string   `json:"status"`
	FailureMessages []string `json:"failureMessages"`
}
