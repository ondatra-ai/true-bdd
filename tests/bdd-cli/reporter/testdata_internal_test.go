package reporter

// Values reused across the package's tests, named so a change to one
// stays a change to one.
const (
	testSubject = "frontend-integration-playwright-startup"
	testSection = "build-code-test-passes"
	testPlain   = "plain"

	// Names and paths the operation tests assert on, in one place so a
	// change to a fixture's shape stays a change to one line.
	testSectionName = "Acceptance Criteria Quality"
	storyFile       = "docs/product/stories/99.3-rewalk-fixture.yaml"
	refineStoryFile = "docs/product/stories/96.3-summary-service-internals.yaml"
	scratchFile     = "tmp/run/scenarios.yaml"
	opusModel       = "claude-opus-4-8"
	crushModel      = "zhipu-coding/glm-5.2"
	firstAC         = "99.3-001"

	// Engine log field names. The log's key spelling is a wire contract
	// the reporter parses against, so the tests name it once.
	fieldCommand     = "command"
	fieldFile        = "file"
	fieldSubjectID   = "subjectID"
	fieldPromptIndex = "promptIndex"
	fieldIteration   = "iteration"
	fieldSection     = "section"
	fieldPath        = "path"
)
