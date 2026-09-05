package steps

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoOrdinalRemote is returned when a clause names a remote or a row by a
// position no Given started.
var ErrNoOrdinalRemote = errors.New("no Given started a remote at that position")

// ErrNoClickedSession is returned when a clause is about the session whose row
// was opened and no When clicked one.
var ErrNoClickedSession = errors.New("no When clicked a row's control")

// ErrNoSessionVersion is returned when the registry entry carries no version,
// so what the row renders cannot be graded against it.
var ErrNoSessionVersion = errors.New("the session's registry entry carries no version")

const (
	// sessionIDAttribute is where a row renders which session it is — the same
	// attribute sessionRow() looks a row up by.
	sessionIDAttribute = "data-session-id"
	// sessionIDKey is that attribute as a step selector writes it.
	sessionIDKey = "session-id"
	// refreshTimeout caps waiting for the list to read the registry again: the
	// page polls on its own clock, not on the step's.
	refreshTimeout = 30 * time.Second
)

// registerSessionsHomeSteps binds the sessions home's vocabulary: the remotes a
// scenario runs in trees of their own, the rows they get, what each row states,
// where its control leads, and the order the rows keep.
func registerSessionsHomeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(one|two|three|four) remotes are running in their own project trees$`,
		startRemotesInOwnTrees)
	suite.Step(`^a second remote is running in its own project tree$`,
		startSecondRemoteInOwnTree)
	suite.Step(`^the (first|second|third|fourth) remote is stopped with "([^"]+)"$`,
		stopOrdinalRemote)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) has "([^"]*)" open$`,
		hasPathOpen)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) clicks the `+
		`(first|second|third|fourth) row's ([a-z][a-z0-9-]*)$`, clickRowControl)
	suite.Step(`^every connected session has its own row$`, assertEverySessionHasRow)
	suite.Step(`^the (first|second|third|fourth) session has its own row$`,
		assertOrdinalSessionHasRow)
	suite.Step(`^(`+selectorPattern+`) has the session's (folder|id|version)$`,
		assertSessionFieldText)
	suite.Step(`^every ([a-z][a-z0-9-]*) is an anchor$`, assertEveryControlIsAnchor)
	suite.Step(`^every ([a-z][a-z0-9-]*)'s href is its own row's workspace home$`,
		assertEveryControlHrefIsOwnHome)
	suite.Step(`^the URL is that session's workspace home$`, assertURLIsClickedSessionHome)
	suite.Step(`^the rows read oldest first by connect time$`, assertRowsOldestFirst)
	suite.Step(`^the rows still read oldest first after a (live|second live) refresh$`,
		assertRowsOldestFirstAfterRefresh)
	registerSessionsMarkerSteps(suite)
	registerSessionsFrameSteps(suite)
	registerSessionsReadSteps(suite)
	registerTimedPresenceSteps(suite)
}

// ordinalLabels are the labels a remote and its session are filed under when a
// scenario starts several and names none.
func ordinalLabels() []string {
	return []string{"first", "second", "third", "fourth"}
}

// ordinalIndex is the 0-based position an ordinal word names.
func ordinalIndex(word string) (int, error) {
	for index, label := range ordinalLabels() {
		if label == word {
			return index, nil
		}
	}

	return 0, fmt.Errorf("%w: %q", ErrNoOrdinalRemote, word)
}

// ordinalCount is how many remotes a counting word asks for.
func ordinalCount(word string) (int, error) {
	count := slices.Index([]string{"", "one", "two", "three", "four"}, word)
	if count < 1 {
		return 0, fmt.Errorf("%w: %q remotes", ErrNoOrdinalRemote, word)
	}

	return count, nil
}

// workspaceHome is the route a session's own workspace hangs under — the same
// per-session path showSessionPage navigates to.
func workspaceHome(sessionID string) string {
	return sessionRoute + sessionID
}

// startRemotesInOwnTrees starts one remote per project tree of its own, filed
// under the position later clauses name it by, oldest first.
func startRemotesInOwnTrees(state *State, args []string) error {
	count, err := ordinalCount(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	for index := range count {
		err = startOrdinalRemote(state, index)
		if err != nil {
			return err
		}
	}

	return nil
}

// startSecondRemoteInOwnTree is the live-arrival When: a remote joining a list
// already on screen, in a tree of its own.
func startSecondRemoteInOwnTree(state *State, _ []string) error {
	return startOrdinalRemote(state, 1)
}

// startOrdinalRemote materializes a tree of this remote's own, starts it there
// and waits for the relay to list it: a Given whose session is unresolved
// leaves every later clause racing the registration.
func startOrdinalRemote(state *State, index int) error {
	labels := ordinalLabels()
	if index >= len(labels) {
		return state.fail("%w: %d", ErrNoOrdinalRemote, index+1)
	}

	label := labels[index]

	tree, err := materializeTree(state.Harness, state.Scenario.ID+"-"+label, workspaceFixture)
	if err != nil {
		return state.fail("preparing the %q project tree for the %s remote: %w",
			workspaceFixture, label, err)
	}

	remote, err := launchRemote(state, tree.Dir)
	if err != nil {
		return err
	}

	state.Remotes[label] = remote

	if state.Tree == nil {
		state.Tree = tree
	}

	if state.Remote == nil {
		state.Remote = remote
	}

	_, err = ensureNamedSession(state, label)

	return err
}

// stopOrdinalRemote signals the remote a scenario named by position, keeping the
// entry it was about: the clauses after it are about a session the relay is
// going to drop.
func stopOrdinalRemote(state *State, args []string) error {
	label, name := args[0], args[1]

	session, err := ensureNamedSession(state, label)
	if err != nil {
		return err
	}

	state.StoppedSession = session

	return stopNamedRemoteWithSignal(state, []string{label, name})
}

// hasPathOpen is the Given form of opening a path: the page is opened with the
// request probe, the document mark and the marker watch installed, which the
// clauses about what never happened read.
func hasPathOpen(state *State, args []string) error {
	page, err := state.Context.NewPage()
	if err != nil {
		return state.fail("open a page: %w", err)
	}

	err = observeRequests(state, page)
	if err != nil {
		return err
	}

	err = observeMarkers(state, page)
	if err != nil {
		return err
	}

	err = observeSessionsGate(state, page)
	if err != nil {
		return err
	}

	state.Page = page
	url := state.RelayURL + args[1]

	_, err = page.Goto(url,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s: %w", ErrNavigation, url, err)
	}

	state.OpenedURL = page.URL()

	return markPageDocument(state, page)
}

// clickRowControl clicks one control inside the row at the named position and
// keeps which session that row was about, which the landing clause names.
func clickRowControl(state *State, args []string) error {
	index, err := ordinalIndex(args[1])
	if err != nil {
		return state.fail("%w", err)
	}

	rememberPageState(state)

	row, err := rowAt(state, index)
	if err != nil {
		return err
	}

	sessionID, err := row.GetAttribute(sessionIDAttribute)
	if err != nil {
		return state.fail("read the %s row's %s: %w", args[1], sessionIDAttribute, err)
	}

	state.ClickedSession = sessionID

	err = row.Locator(elementCSS(args[2], "", "")).Click()
	if err != nil {
		return state.fail("clicking the %s row's %s: %w", args[1], args[2], err)
	}

	return nil
}

// rowAt is the session row at one position, waited for: the list renders off
// its own poll, so the row a clause names is not there on the first read.
func rowAt(state *State, index int) (playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	row := page.Locator(elementCSS(sessionRowTestID, "", "")).Nth(index)

	err = row.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(rowTimeout),
	})
	if err != nil {
		return nil, state.fail("the sessions list never showed a row at position %d: %w\n%s",
			index+1, err, visibleText(page))
	}

	return row, nil
}

// assertEverySessionHasRow holds the list to a row per CONNECTED session, read
// from the registry the list itself polls rather than from what it rendered.
func assertEverySessionHasRow(state *State, _ []string) error {
	sessions, err := listSessions(state.RelayURL)
	if err != nil {
		return state.fail("%w", err)
	}

	if len(sessions) == 0 {
		return state.fail("%w: it lists none, so no session can have a row of its own",
			ErrSessionNotListed)
	}

	for index := range sessions {
		err = assertSessionRowShown(state, sessions[index].SessionID)
		if err != nil {
			return err
		}
	}

	return nil
}

// assertOrdinalSessionHasRow holds the session a Given started at that position
// to having a row of its own.
func assertOrdinalSessionHasRow(state *State, args []string) error {
	session, err := ensureNamedSession(state, args[0])
	if err != nil {
		return err
	}

	return assertSessionRowShown(state, session.SessionID)
}

// assertSessionRowShown holds the list to exactly one row for that session,
// polling: the list re-reads the registry on its own clock.
func assertSessionRowShown(state *State, sessionID string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel := selector{Name: sessionRowTestID, Key: sessionIDKey, Value: sessionID}

	got, matched, err := await(readCount(page, sel), equals("1"))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("the page shows %s rows for session %q, want exactly one\n%s",
			got, sessionID, visibleText(page))
	}

	return nil
}

// assertSessionFieldText holds one cell's rendered text to what the registry
// says about this scenario's session — the same read the list itself polls.
func assertSessionFieldText(state *State, args []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	field := args[1]

	want, err := sessionField(state, session, field)
	if err != nil {
		return err
	}

	sel, locator, err := locateStep(state, args[0])
	if err != nil {
		return err
	}

	got, matched, err := await(readInnerText(locator), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("%s reads %q, want the session's %s, %q", sel, got, field, want)
	}

	return nil
}

// sessionField is what the registry holds for the field a clause names, or the
// reason that field cannot be graded at all.
func sessionField(state *State, session *sessionSummary, field string) (string, error) {
	switch field {
	case folderKey:
		return session.Folder, nil
	case "id":
		return session.SessionID, nil
	default:
		if session.Version == "" {
			return "", state.fail("%w: session %q", ErrNoSessionVersion, session.SessionID)
		}

		return session.Version, nil
	}
}

// assertEveryControlIsAnchor holds every control of that name to being a real
// link — an anchor with a target, which is what a reader can open in a new tab.
func assertEveryControlIsAnchor(state *State, args []string) error {
	probe := fmt.Sprintf(`els => els.map(el => {
		if (el.tagName !== "A") { return "a control is a <" + el.tagName.toLowerCase() + ">" }
		return (el.getAttribute("href") || "") === "" ?
			"an anchor carries no href" : %q
	})`, verdictOK)

	return assertEveryElement(state, elementCSS(args[0], "", ""), probe,
		"every "+args[0]+" must be an anchor")
}

// assertEveryControlHrefIsOwnHome holds each row's control to pointing into
// THAT row's workspace, so two rows can never share one target.
func assertEveryControlHrefIsOwnHome(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	probe := fmt.Sprintf(`els => els.map(el => {
		const control = el.querySelector(%q)
		return (el.getAttribute(%q) || "") + " " +
			(control ? (control.getAttribute("href") || "") : "")
	})`, elementCSS(args[0], "", ""), sessionIDAttribute)

	rows := page.Locator(elementCSS(sessionRowTestID, "", ""))

	got, matched, err := await(readRowProbe(rows, probe), everyRowTargetsOwnHome)
	if err != nil {
		return state.fail("reading each row's %s: %w", args[0], err)
	}

	if !matched {
		return state.fail("the rows' %s point at %s, want each into its own row's "+
			"workspace home under %q", args[0], describeTargets(got), sessionRoute)
	}

	return nil
}

// everyRowTargetsOwnHome answers whether every row's control leads into that
// row's own workspace, and refuses an empty reading: a clause about every row
// is not satisfied by a page that renders none.
func everyRowTargetsOwnHome(reading string) bool {
	if reading == "" {
		return false
	}

	for _, line := range strings.Split(reading, "\n") {
		sessionID, href, _ := strings.Cut(line, linkFieldSeparator)
		if !targetsWorkspaceHome(href, sessionID) {
			return false
		}
	}

	return true
}

// targetsWorkspaceHome answers whether a target leads into that session's own
// workspace: its home, or a page under it.
func targetsWorkspaceHome(href, sessionID string) bool {
	if sessionID == "" || href == "" {
		return false
	}

	home := workspaceHome(sessionID)

	return href == home || strings.HasPrefix(href, home+"/") ||
		strings.HasPrefix(href, home+"?")
}

// describeTargets renders what each row DID point at, so a failure carries the
// alternative rather than only the rule.
func describeTargets(reading string) string {
	if reading == "" {
		return "nothing — the page shows no rows"
	}

	lines := strings.Split(reading, "\n")
	parts := make([]string, 0, len(lines))

	for _, line := range lines {
		sessionID, href, _ := strings.Cut(line, linkFieldSeparator)
		parts = append(parts, fmt.Sprintf("session %q -> %q", sessionID, href))
	}

	return strings.Join(parts, "; ")
}

// assertURLIsClickedSessionHome holds where the click landed to the workspace
// of the row it was made on.
func assertURLIsClickedSessionHome(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	if state.ClickedSession == "" {
		return state.fail("%w", ErrNoClickedSession)
	}

	sessionID := state.ClickedSession

	got, matched, err := await(readURL(page), func(value string) bool {
		return targetsWorkspaceHome(strings.TrimPrefix(value, state.RelayURL), sessionID)
	})
	if err != nil {
		return state.fail("reading the page URL: %w", err)
	}

	if !matched {
		return state.fail("the URL is %q, want session %q's workspace home, %q",
			got, sessionID, state.RelayURL+workspaceHome(sessionID))
	}

	return nil
}

// assertRowsOldestFirst holds the rendered order to the registry's connect
// times, oldest first.
func assertRowsOldestFirst(state *State, _ []string) error {
	return assertRowOrder(state)
}

// assertRowsOldestFirstAfterRefresh waits for the list to read the registry
// again and holds the order across that read: an order surviving only the first
// render is not an order.
func assertRowsOldestFirstAfterRefresh(state *State, _ []string) error {
	err := awaitSessionsRead(state)
	if err != nil {
		return err
	}

	return assertRowOrder(state)
}

// assertRowOrder compares the ids the rows render, in DOM order, with when the
// registry says each session connected.
func assertRowOrder(state *State) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	times, err := connectTimes(state)
	if err != nil {
		return err
	}

	rows := page.Locator(elementCSS(sessionRowTestID, "", ""))
	probe := fmt.Sprintf(`els => els.map(el => el.getAttribute(%q) || "")`, sessionIDAttribute)

	got, matched, err := await(readRowProbe(rows, probe), oldestFirst(times))
	if err != nil {
		return state.fail("reading the rows' order: %w", err)
	}

	if !matched {
		return state.fail("the rows read %q, want the connected sessions oldest first, %q",
			strings.ReplaceAll(got, "\n", ", "), strings.Join(oldestFirstIDs(times), ", "))
	}

	return nil
}

// connectTimes is when the registry says each connected session arrived.
func connectTimes(state *State) (map[string]int64, error) {
	sessions, err := listSessions(state.RelayURL)
	if err != nil {
		return nil, state.fail("%w", err)
	}

	if len(sessions) == 0 {
		return nil, state.fail("%w: it lists none to order", ErrSessionNotListed)
	}

	times := make(map[string]int64, len(sessions))
	for index := range sessions {
		times[sessions[index].SessionID] = sessions[index].ConnectedAt
	}

	return times, nil
}

// oldestFirst accepts a rendered order holding every connected session with
// connect times that never go back. Ties are left free: two sessions the
// registry timestamps alike have no older one.
func oldestFirst(times map[string]int64) func(string) bool {
	return func(reading string) bool {
		if reading == "" {
			return false
		}

		ids := strings.Split(reading, "\n")
		if len(ids) != len(times) {
			return false
		}

		previous := int64(0)

		for position, sessionID := range ids {
			arrived, listed := times[sessionID]
			if !listed || (position > 0 && arrived < previous) {
				return false
			}

			previous = arrived
		}

		return true
	}
}

// oldestFirstIDs is the registry's own order, so a failure names what the rows
// should have read.
func oldestFirstIDs(times map[string]int64) []string {
	ids := make([]string, 0, len(times))
	for sessionID := range times {
		ids = append(ids, sessionID)
	}

	sort.SliceStable(ids, func(one, two int) bool {
		if times[ids[one]] == times[ids[two]] {
			return ids[one] < ids[two]
		}

		return times[ids[one]] < times[ids[two]]
	})

	return ids
}

// awaitSessionsRead waits for the page to read the registry once more, so the
// order clause after it is about a list that re-rendered rather than a stale one.
func awaitSessionsRead(state *State) error {
	before := sessionsReadCount(state)
	deadline := time.Now().Add(refreshTimeout)

	for time.Now().Before(deadline) {
		if sessionsReadCount(state) > before {
			return nil
		}

		time.Sleep(valuePollInterval)
	}

	return state.fail("the page never read %s again within %s; it issued %s",
		sessionsPath, refreshTimeout, requestSummary(state))
}

// sessionsReadCount is how many times the page has read the registry.
func sessionsReadCount(state *State) int {
	requests, err := pageRequests(state)
	if err != nil {
		return 0
	}

	count := 0

	for _, request := range requests {
		if request.Method == http.MethodGet && strings.Contains(request.URL, sessionsPath) {
			count++
		}
	}

	return count
}

// readRowProbe reads one string per session row, joined one per line, so the
// row clauses poll through the same await every other value clause uses.
func readRowProbe(locator playwright.Locator, probe string) func() (string, error) {
	return func() (string, error) {
		values, err := evaluateAllStrings(locator, probe)
		if err != nil {
			return "", err
		}

		return strings.Join(values, "\n"), nil
	}
}
