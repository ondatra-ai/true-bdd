package steps

import (
	"errors"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrRelayNotRunning is returned when a scenario asserts the relay is up
// and the harness never got it answering.
var ErrRelayNotRunning = errors.New("the relay is not running")

// ErrNavigation is returned when a page did not load.
var ErrNavigation = errors.New("navigation failed")

// textTimeout caps how long a Then step waits for text to appear. The
// page is already loaded by the time one runs, so this is the budget for
// hydration and a client render, not for the network.
const textTimeout = 10_000 // milliseconds

// Register binds every step this suite's scenarios can use — the same
// shape as the CLI suite's Register, deliberately a different vocabulary
// ("opens", "shows", "the page title is") so a step names its surface.
func Register(suite *bddgo.Suite[State]) {
	suite.Step(`^the relay is running$`, relayIsRunning)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) opens "([^"]*)"$`, openPath)
	suite.Step(`^the page title is "([^"]*)"$`, assertTitle)
	suite.Step(`^the page shows "([^"]*)"$`, assertText)
	// The protocol vocabulary: a host project, a remote running in it, and
	// what the registry and the sessions list then say about the session it
	// registered. Implemented in session_steps.go.
	suite.Step(`^the "([^"]+)" project tree$`, prepareProjectTree)
	suite.Step(`^a remote is running through a symlink to its project tree$`,
		startRemoteThroughSymlink)
	suite.Step(`^the session is listed$`, assertSessionListed)
	suite.Step(`^the session's folder is the project tree's real path$`,
		assertSessionFolderIsRealPath)
	suite.Step(`^the session's row shows the project tree's real path$`,
		assertRowShowsRealPath)
	suite.Step(`^the session's row does not show the symlink path$`,
		assertRowHidesSymlinkPath)
	// The run vocabulary: a remote in the tree, a dispatch the scenario
	// labels, the signal that disconnects it, and the HTTP surface every
	// clause about a gone session reads. Implemented in api_steps.go.
	suite.Step(`^a remote is running$`, startRemoteInTree)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" on the session as run "([^"]+)"$`,
		dispatchRun)
	suite.Step(`^the remote is stopped with "([^"]+)"$`, stopRemoteWithSignal)
	suite.Step(`^the session is not listed$`, assertSessionNotListed)
	suite.Step(`^a GET to "([^"]+)" returns status (\d+)$`, assertGetStatus)
	suite.Step(
		`^a POST to "([^"]+)" with the dispatch body for command "([^"]*)" returns status (\d+)$`,
		assertDispatchStatus)
	suite.Step(`^the response names error "([^"]+)"$`, assertResponseError)
	// The run-lifecycle vocabulary: what a dispatched run publishes, and what
	// re-sending its token answers. Implemented in run_steps.go.
	suite.Step(`^run "([^"]+)" publishes a "([^"]+)" prompt$`, assertPromptPublished)
	suite.Step(`^dispatching the same token returns run "([^"]+)" with status (\d+)$`,
		assertSameTokenReturnsRun)
	// The session page's rendered surface: one selector grammar
	// (`name[key=value] > child[key=value]`) spliced into every clause below.
	// Implemented in page_steps.go.
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) opens the session page$`,
		openSessionPage)
	suite.Step(`^the page shows (`+selectorPattern+`)$`, assertElementShown)
	// The keyed-child presence clause: same assertion, disjoint pattern —
	// the child's bracket is required here and impossible above, so a step
	// matches exactly one of the two.
	suite.Step(`^the page shows (`+keyedChildSelectorPattern+`)$`, assertElementShown)
	suite.Step(`^(`+selectorPattern+`) has attribute "([^"]*)" = "([^"]*)"$`, assertAttribute)
	suite.Step(`^(`+selectorPattern+`) has text "([^"]*)"$`, assertElementText)
	suite.Step(`^(`+selectorPattern+`) is enabled$`, assertEnabled)
	suite.Step(`^(`+selectorPattern+`) is disabled$`, assertDisabled)
	suite.Step(`^(`+selectorPattern+`) has the "([^"]*)" of "([^"]*)"$`, assertFieldOfFile)
	suite.Step(`^(`+selectorPattern+`) has the "([^"]*)" of story "([^"]*)" of "([^"]*)"$`,
		assertFieldOfStory)
	// The interaction vocabulary over the same grammar: the page already open
	// as a Given, a click, and the absence clause that reads what the click
	// did. Implemented in page_steps.go.
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) has the session page open$`,
		openSessionPage)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) clicks (`+selectorPattern+`)$`,
		clickElement)
	suite.Step(`^the page does not show (`+selectorPattern+`)$`, assertElementNotShown)
	suite.Step(`^the page shows exactly (\d+) (`+selectorPattern+`)$`, assertElementCount)
	// The registry-mutation vocabulary: a change written to the project tree
	// outside the browser, which a refresh must pick up. Implemented in
	// registry_steps.go.
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) appends a scenario covering "([^"]+)" to "([^"]+)"$`,
		appendCoveringScenario)
	registerNamedSessionSteps(suite)
	registerRelayProcessSteps(suite)
	registerRelayRequestSteps(suite)
	registerPortedSteps(suite)
}

// registerPortedSteps binds the relay and run vocabularies ported behind the
// first slice — restart survival, a run's lifecycle, the agent work queue —
// and hands the workspace half to registerPortedWebSteps.
func registerPortedSteps(suite *bddgo.Suite[State]) {
	registerRestartSteps(suite)
	registerRunLifecycleSteps(suite)
	registerModalSteps(suite)
	registerStoryOracleSteps(suite)
	registerAgentWorkSteps(suite)
	registerTreeEditSteps(suite)
	registerAgentEpochSteps(suite)
	registerPromptAnswerSteps(suite)
	registerConcurrentReadSteps(suite)
	registerPromptProgressSteps(suite)
	registerCrossSessionAnswerSteps(suite)
	registerCrossRelaySteps(suite)
	registerCrossRelayWorkSteps(suite)
	registerRedisSteps(suite)
	registerRunStreamSteps(suite)
	registerSweepSteps(suite)

	registerPortedWebSteps(suite)
}

// registerPortedWebSteps binds the workspace half of that port: documents,
// layout, the design assertions and the tables. Split from its caller only to
// stay inside the statement ceiling.
func registerPortedWebSteps(suite *bddgo.Suite[State]) {
	registerWorkspaceSteps(suite)
	registerFileViewSteps(suite)
	registerEditorSteps(suite)
	registerDocumentEditSteps(suite)
	registerDocWriteSteps(suite)
	registerProductDocumentSteps(suite)
	registerProductOutlineSteps(suite)
	registerMultiDocumentEditSteps(suite)
	registerNewStorySteps(suite)
	registerFeatureTaggingSteps(suite)
	registerServedProductSteps(suite)
	registerChatDockSteps(suite)
	registerLayoutSteps(suite)
	registerOverviewSteps(suite)
	registerOverviewStateSteps(suite)
	registerBreadcrumbSteps(suite)
	registerPageTextSteps(suite)
	registerElementShapeSteps(suite)
	registerDesignTokenSteps(suite)
	registerDesignMeasureSteps(suite)
	registerDesignScaleSteps(suite)
	registerPageScrollSteps(suite)
	registerRailSteps(suite)
	registerFileCardSteps(suite)
	registerFeatureRowSteps(suite)
	registerScenarioTableSteps(suite)
	registerSessionsHomeSteps(suite)
	registerAIRunSteps(suite)
}

// registerAIRunSteps binds the vocabulary the AI scenarios share: a fix dispatch,
// answers to its prompts, what the run reads back as, what it did to the tree,
// what it spent, and the session left behind. One call, for its caller's ceiling.
func registerAIRunSteps(suite *bddgo.Suite[State]) {
	registerAIDispatchSteps(suite)
	registerPromptChoiceSteps(suite)
	registerRunFactSteps(suite)
	registerTreeChangeSteps(suite)
	registerAICallSteps(suite)
	registerSessionRunSteps(suite)
	registerStoryRowSteps(suite)
	registerElementCompareSteps(suite)
	registerRunArtifactSteps(suite)
	registerRegistryBlockSteps(suite)
	registerProjectTestsSteps(suite)
	registerRemoteSignalSteps(suite)
	registerProjectHistorySteps(suite)
	registerCommandChildSteps(suite)
}

// registerNamedSessionSteps binds the vocabulary a scenario running more than
// one remote uses: each session addressed by the label its Given gave it.
// Implemented in named_session_steps.go.
func registerNamedSessionSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^a remote is running as "([^"]+)"$`, startNamedRemote)
	suite.Step(`^a remote is running as "([^"]+)" in the same project tree$`, startSiblingRemote)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches `+
			`"([^"]+)" on session "([^"]+)" as run "([^"]+)"$`,
		dispatchRunOnNamedSession)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) opens the session page of "([^"]+)"$`,
		openNamedSessionPage)
	suite.Step(`^remote "([^"]+)" is stopped with "([^"]+)"$`, stopNamedRemoteWithSignal)
	suite.Step(`^remote "([^"]+)" is running$`, assertNamedRemoteRunning)
}

// relayIsRunning asserts the precondition the harness already
// established, so a scenario states it rather than assuming it.
func relayIsRunning(state *State, _ []string) error {
	if state.RelayURL == "" {
		return state.fail("%w", ErrRelayNotRunning)
	}

	return nil
}

// openPath navigates the scenario's page to a path on the relay. The
// captured role is discarded — held to the product document's role list
// by the pattern itself.
func openPath(state *State, args []string) error {
	path := args[1]

	page, err := state.Context.NewPage()
	if err != nil {
		return state.fail("open a page: %w", err)
	}

	err = observeRequests(state, page)
	if err != nil {
		return err
	}

	err = observeSessionsGate(state, page)
	if err != nil {
		return err
	}

	state.Page = page

	_, err = page.Goto(state.RelayURL+path,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s%s: %w", ErrNavigation, state.RelayURL, path, err)
	}

	return nil
}

func assertTitle(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	got, err := page.Title()
	if err != nil {
		return state.fail("read the page title: %w", err)
	}

	if got != want {
		return state.fail("page title is %q, want %q", got, want)
	}

	return nil
}

// assertText waits for the text to appear, rather than reading once: the
// page is server-rendered but hydrates client-side, and asserting at
// DOMContentLoaded is how a browser suite acquires the flake it blames on the browser.
func assertText(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	want := args[0]

	locator := page.GetByText(want)

	err = locator.First().WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(textTimeout),
	})
	if err != nil {
		return state.fail("the page never showed %q: %w\n%s", want, err, visibleText(page))
	}

	return nil
}

// visibleText renders what the page DID show, so a missing-text failure
// carries the alternative instead of sending the reader to a screenshot.
func visibleText(page playwright.Page) string {
	body, err := page.Locator(bodyKey).InnerText()
	if err != nil {
		return "(the page's text could not be read)"
	}

	return "--- page text ---\n" + strings.TrimSpace(body)
}
