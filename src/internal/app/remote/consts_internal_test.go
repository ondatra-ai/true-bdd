package remote

// Shared test fixtures for the remote package's internal tests, kept in
// one place so repeated literals do not trip goconst.
const (
	testSessionID = "session-1"
	testRunID     = "run-1"

	// kindChoice is the "choice" prompt kind reused across projection tests.
	kindChoice = "choice"
	// capToken is the fake capability token the loop test's relay hands out.
	capToken = "cap"
	// testPromptID is the pending-prompt id the projection tests drive through
	// the store, assert on, and answer.
	testPromptID = "prompt-1"
)
