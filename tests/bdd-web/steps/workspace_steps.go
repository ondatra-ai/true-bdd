package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoWorkspaceTarget is returned when a step names a workspace this suite
// has no route for.
var ErrNoWorkspaceTarget = errors.New("no workspace route for that name")

// ErrNoOpenedWorkspace is returned when a clause is about the URL not
// changing and no earlier step opened a workspace.
var ErrNoOpenedWorkspace = errors.New("no step opened a workspace")

// ErrNotAppRouter is returned when the open document carries no App Router
// payload.
var ErrNotAppRouter = errors.New("the route is not served by an App Router module")

const (
	// workspaceFixture is the host project every workspace scenario is
	// written against: the tree the connected remote runs in.
	workspaceFixture = "workspace-host"
	// sidebarCaretTestID is the toggle a sidebar group reveals on hover, and
	// expandedAttribute is where that toggle renders the group's state.
	sidebarCaretTestID = "sidebar-caret"
	expandedAttribute  = "data-expanded"
	// groupCollapsed is what the attribute reads once the group is shut.
	groupCollapsed = "false"
	// appRouterKind, pagesRouterKind and unknownRouterKind name which Next.js
	// router served the document: App Router streams a flight payload into
	// self.__next_f, the Pages Router embeds a __NEXT_DATA__ script.
	appRouterKind     = "app-router"
	pagesRouterKind   = "pages-router"
	unknownRouterKind = "neither"
)

// registerWorkspaceSteps binds the workspace shell's vocabulary: the
// connected project it serves, the named targets it opens, and the URL and
// routing clauses about where a navigation landed.
func registerWorkspaceSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the workspace is connected$`, connectWorkspace)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) opens the workspace "([^"]+)"$`,
		openWorkspace)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has the workspace "([^"]+)" open$`,
		openWorkspace)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) hovers (`+selectorPattern+`)$`,
		hoverElement)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) has collapsed (`+selectorPattern+`)$`,
		collapseGroup)
	suite.Step(`^the URL ends with "([^"]*)"$`, assertURLEndsWith)
	suite.Step(`^the URL contains "([^"]*)"$`, assertURLContains)
	suite.Step(`^the URL did not change$`, assertURLUnchanged)
	suite.Step(`^the route is served by an App Router module$`, assertAppRouterRoute)
	suite.Step(`^the viewport is (\d+)x(\d+)$`, setViewport)
}

// connectWorkspace is the Given every workspace scenario opens with: the
// host project the workspace serves, a remote running in it, and the relay
// listing the session that remote registered.
func connectWorkspace(state *State, _ []string) error {
	return connectWorkspaceWith(state)
}

// connectWorkspaceWith is that connection with extra settings on the remote's
// environment, which the chat driver's Given adds one of.
func connectWorkspaceWith(state *State, env ...string) error {
	err := prepareProjectTree(state, []string{workspaceFixture})
	if err != nil {
		return err
	}

	err = attachRemote(state, state.Tree.Dir, env...)
	if err != nil {
		return err
	}

	_, err = ensureSession(state)

	return err
}

// openWorkspace navigates to the route a named target resolves to. The Given
// that has one open and the When that moves to another are one navigation, so
// both bind here; the captured role is discarded, as openPath's is.
func openWorkspace(state *State, args []string) error {
	route, err := workspaceRoute(state, args[1])
	if err != nil {
		return err
	}

	page, err := workspacePage(state)
	if err != nil {
		return err
	}

	url := state.RelayURL + route

	_, err = page.Goto(url,
		playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return state.fail("%w: %s: %w", ErrNavigation, url, err)
	}

	state.OpenedURL = page.URL()

	return nil
}

// workspaceRoute maps the name a step writes onto the route the workspace
// serves it at. An unknown name is refused rather than guessed at: a made-up
// route is a 404 the reader would have to diagnose.
func workspaceRoute(state *State, target string) (string, error) {
	if storyID, ok := strings.CutPrefix(target, "story "); ok {
		return "/product/stories/" + storyID, nil
	}

	if featureID, ok := strings.CutPrefix(target, "feature "); ok {
		return "/product/features/" + featureID, nil
	}

	switch target {
	case "home", architectureNode, productNode, "builds":
		return "/" + target, nil
	case featuresNode, scenariosNode:
		return "/product/" + target, nil
	default:
		return "", state.fail("%w: %q", ErrNoWorkspaceTarget, target)
	}
}

// workspacePage is the scenario's page, opened on first use: a scenario that
// opens a second workspace navigates the page it already has.
func workspacePage(state *State) (playwright.Page, error) {
	if state.Page != nil {
		return state.Page, nil
	}

	page, err := state.Context.NewPage()
	if err != nil {
		return nil, state.fail("open a page: %w", err)
	}

	err = observeWrites(state, page)
	if err != nil {
		return nil, err
	}

	err = observeRequests(state, page)
	if err != nil {
		return nil, err
	}

	state.Page = page

	return page, nil
}

// setViewport sizes the browser before the page is navigated, so a scenario
// about a layout at one width states that width rather than inheriting the
// context's. The page is opened here; the navigation that follows reuses it.
func setViewport(state *State, args []string) error {
	width, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step's width %q does not parse: %w", args[0], err)
	}

	height, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step's height %q does not parse: %w", args[1], err)
	}

	page, err := workspacePage(state)
	if err != nil {
		return err
	}

	err = page.SetViewportSize(width, height)
	if err != nil {
		return state.fail("sizing the viewport to %dx%d: %w", width, height, err)
	}

	return nil
}

// hoverElement is the pointer-only When: a rail entry's flyout and a sidebar
// group's caret are revealed by hover and by nothing else.
func hoverElement(state *State, args []string) error {
	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = locator.Hover()
	if err != nil {
		return state.fail("hovering %s: %w", sel, err)
	}

	return nil
}

// collapseGroup shuts one sidebar group as a Given: hover reveals the caret,
// the click toggles it, and the wait holds the state to having taken — an
// unverified precondition is not a precondition.
func collapseGroup(state *State, args []string) error {
	sel, group, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	err = group.Hover()
	if err != nil {
		return state.fail("hovering %s: %w", sel, err)
	}

	caret := group.Locator(elementCSS(sidebarCaretTestID, "", ""))

	err = caret.Click()
	if err != nil {
		return state.fail("clicking %s > %s: %w", sel, sidebarCaretTestID, err)
	}

	got, matched, err := await(readAttribute(caret, expandedAttribute), equals(groupCollapsed))
	if err != nil {
		return state.fail("%s > %s: %w", sel, sidebarCaretTestID, err)
	}

	if !matched {
		return state.fail("%s > %s has %s = %q after the collapse, want %q",
			sel, sidebarCaretTestID, expandedAttribute, got, groupCollapsed)
	}

	return nil
}

// assertURLEndsWith holds the browser's URL to ending in the step's text,
// polling: a click navigates on the client, so the first read after it is
// the URL the navigation has not reached yet.
func assertURLEndsWith(state *State, args []string) error {
	return assertURL(state, args[0], "ends with", strings.HasSuffix)
}

// assertURLContains is the same clause for a route whose tail carries an id
// the scenario does not name.
func assertURLContains(state *State, args []string) error {
	return assertURL(state, args[0], "contains", strings.Contains)
}

// assertURL is the shared poll; relation names the comparison in the failure,
// so the reader is told which one did not hold.
func assertURL(state *State, want, relation string,
	matches func(string, string) bool,
) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := await(readURL(page),
		func(value string) bool { return matches(value, want) })
	if err != nil {
		return state.fail("reading the page URL: %w", err)
	}

	if !matched {
		return state.fail("the URL is %q, want one that %s %q", got, relation, want)
	}

	return nil
}

// readURL reads the browser's current URL as a reader, so the URL clauses
// poll through the same await every other value clause uses.
func readURL(page playwright.Page) func() (string, error) {
	return func() (string, error) { return page.URL(), nil }
}

// assertURLUnchanged holds the URL to the one the workspace was opened on —
// the clause a scenario about an interaction that must NOT navigate ends with,
// so it reads once rather than waiting for a change it forbids.
func assertURLUnchanged(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	if state.OpenedURL == "" {
		return state.fail("%w", ErrNoOpenedWorkspace)
	}

	got := page.URL()
	if got != state.OpenedURL {
		return state.fail("the URL is %q, want the one the workspace was opened on, %q",
			got, state.OpenedURL)
	}

	return nil
}

// assertAppRouterRoute holds the served route to being an App Router module,
// read from what the framework itself leaves in the document rather than from
// the source tree, which this suite never sees.
func assertAppRouterRoute(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := await(readRouterKind(page), equals(appRouterKind))
	if err != nil {
		return state.fail("%w: %w", ErrNotAppRouter, err)
	}

	if !matched {
		return state.fail("%w: %s reads as %s, want %s",
			ErrNotAppRouter, page.URL(), got, appRouterKind)
	}

	return nil
}

// readRouterKind names which Next.js router served the open document.
func readRouterKind(page playwright.Page) func() (string, error) {
	probe := fmt.Sprintf(`() => Array.isArray(globalThis.__next_f) ? %q : `+
		`(document.getElementById("__NEXT_DATA__") ? %q : %q)`,
		appRouterKind, pagesRouterKind, unknownRouterKind)

	return func() (string, error) {
		value, err := page.Evaluate(probe)
		if err != nil {
			return "", fmt.Errorf("read which router served the page: %w", err)
		}

		kind, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%w: the router probe answered %v", ErrNotAppRouter, value)
		}

		return kind, nil
	}
}
