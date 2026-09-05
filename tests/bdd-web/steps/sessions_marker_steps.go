package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoMarkerWatch is returned when a clause is about what never appeared and
// no Given opened the page with the watch installed.
var ErrNoMarkerWatch = errors.New(
	`no Given opened the page as 'has "<path>" open', which installs the marker watch`)

// ErrNoStoppedSession is returned when a clause is about the stopped session
// and no When stopped a remote.
var ErrNoStoppedSession = errors.New("no When stopped a remote")

// ErrPageNavigated is returned when the document a Given opened was replaced.
var ErrPageNavigated = errors.New("the page navigated")

const (
	// documentMark is stamped on the open document after it loads; a reload or
	// a navigation replaces the window and takes it with it.
	documentMark = "true-bdd-document-mark"
	// markerWatchAbsent is what the read answers when no Given installed the
	// watch, which is a step failure rather than a clean page.
	markerWatchAbsent = "watch-absent"
	// markerKindText and markerKindControl split a sighting in the page's prose
	// from one on a control, which the two clauses read apart.
	markerKindText    = "text"
	markerKindControl = "control"
	// markerWatchScript records every sighting of the retired disconnection
	// vocabulary from first paint on, which is what "at any point" needs.
	markerWatchScript = `(() => {
	  if (window.__tbddMarkerWatch) { return }
	  const seen = []
	  window.__tbddMarkerWatch = seen
	  const pattern = /disconnect|unreachable|reconnect/i
	  const note = (kind, text) => {
	    const record = kind + " " +
	      String(text).trim().replace(/\s+/g, " ").slice(0, 200)
	    if (!seen.includes(record)) { seen.push(record) }
	  }
	  const scan = () => {
	    const body = document.body
	    if (!body) { return }
	    const text = body.innerText || ""
	    if (pattern.test(text)) { note("text", text) }
	    const controls = body.querySelectorAll(
	      "a, button, input, select, [role=button], [role=link], [aria-label]")
	    for (const el of controls) {
	      const label = (el.getAttribute("aria-label") || "") + " " +
	        (el.getAttribute("value") || "") + " " + (el.textContent || "")
	      if (pattern.test(label)) {
	        note("control", el.tagName.toLowerCase() + ": " + label)
	      }
	    }
	  }
	  const start = () => {
	    scan()
	    new MutationObserver(scan).observe(document.documentElement,
	      {subtree: true, childList: true, characterData: true, attributes: true})
	  }
	  if (document.body) { start() } else {
	    document.addEventListener("DOMContentLoaded", start)
	  }
	})()`
	// documentMarkScript stamps the loaded document and markerReadProbe and
	// documentMarkProbe read the two back.
	documentMarkScript = `() => { window.__tbddDocumentMark = "` + documentMark + `" }`
	documentMarkProbe  = `() => window.__tbddDocumentMark || ""`
	markerReadProbe    = `() => window.__tbddMarkerWatch ? ` +
		`window.__tbddMarkerWatch.join("\n") : "` + markerWatchAbsent + `"`
)

// registerSessionsMarkerSteps binds the clauses about what the sessions home
// must NEVER have done: navigated, kept a gone session's id, or offered the
// retired disconnection vocabulary.
func registerSessionsMarkerSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the page never navigated$`, assertNeverNavigated)
	suite.Step(`^nothing on the page carries the stopped session's id$`,
		assertStoppedSessionGoneFromPage)
	suite.Step(`^no disconnected, unreachable or reconnect marker appeared at any point$`,
		assertNoMarkerAppeared)
	suite.Step(`^no control was ever labelled with those words$`, assertNoControlLabelled)
}

// observeMarkers installs the watch BEFORE the page navigates, which is the
// only moment from which "at any point" can be answered.
func observeMarkers(state *State, page playwright.Page) error {
	err := page.AddInitScript(playwright.Script{
		Content: playwright.String(markerWatchScript),
	})
	if err != nil {
		return state.fail("installing the marker watch: %w", err)
	}

	return nil
}

// markPageDocument stamps the loaded document, so a later clause tells a live
// re-render from a reload that lands on the same URL.
func markPageDocument(state *State, page playwright.Page) error {
	_, err := page.Evaluate(documentMarkScript)
	if err != nil {
		return state.fail("marking the open document: %w", err)
	}

	return nil
}

// assertNeverNavigated holds the page to being the very document the Given
// opened: same window, same URL.
func assertNeverNavigated(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	if state.OpenedURL == "" {
		return state.fail("%w", ErrNoOpenedWorkspace)
	}

	got, err := probeString(page, documentMarkProbe)
	if err != nil {
		return state.fail("reading the document mark: %w", err)
	}

	if got != documentMark {
		return state.fail("%w: the document the Given marked is gone, so the page reloaded",
			ErrPageNavigated)
	}

	if page.URL() != state.OpenedURL {
		return state.fail("%w: the URL is %q, want the one it was opened on, %q",
			ErrPageNavigated, page.URL(), state.OpenedURL)
	}

	return nil
}

// assertStoppedSessionGoneFromPage waits for every trace of the stopped session
// to leave the rendered page — markup included, so a hidden row or a stale
// attribute is caught as well as text.
func assertStoppedSessionGoneFromPage(state *State, _ []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	if state.StoppedSession == nil {
		return state.fail("%w", ErrNoStoppedSession)
	}

	sessionID := state.StoppedSession.SessionID
	budget := sessionGoneTimeout + valueTimeout

	_, matched, err := awaitWithin(budget, readContent(page),
		func(html string) bool { return !strings.Contains(html, sessionID) })
	if err != nil {
		return state.fail("reading the page: %w", err)
	}

	if !matched {
		return state.fail("the page still carries the stopped session's id %q after %s\n%s",
			sessionID, budget, visibleText(page))
	}

	return nil
}

// readContent reads the page's whole markup, which is where an id survives a
// row a reader can no longer see.
func readContent(page playwright.Page) func() (string, error) {
	return func() (string, error) {
		html, err := page.Content()
		if err != nil {
			return "", fmt.Errorf("read the page's markup: %w", err)
		}

		return html, nil
	}
}

// assertNoMarkerAppeared holds the whole visit to never having shown the
// retired disconnection vocabulary, read from a watch installed before the
// first paint rather than sampled once at the end.
func assertNoMarkerAppeared(state *State, _ []string) error {
	return assertNoSightings(state, markerKindText,
		"no disconnected, unreachable or reconnect marker may appear")
}

// assertNoControlLabelled reads the same watch for controls: a button or a link
// carrying one of those words is an offer this product does not make.
func assertNoControlLabelled(state *State, _ []string) error {
	return assertNoSightings(state, markerKindControl,
		"no control may be labelled disconnected, unreachable or reconnect")
}

// assertNoSightings reads the watch and fails with what it saw, of that kind.
func assertNoSightings(state *State, kind, wanted string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	reading, err := probeString(page, markerReadProbe)
	if err != nil {
		return state.fail("reading the marker watch: %w", err)
	}

	if reading == markerWatchAbsent {
		return state.fail("%w", ErrNoMarkerWatch)
	}

	sightings := sightingsOfKind(reading, kind)
	if len(sightings) > 0 {
		return state.fail("%s, and the page showed %s", wanted, strings.Join(sightings, "; "))
	}

	return nil
}

// sightingsOfKind is what the watch recorded of one kind, quoted as the reader
// saw it.
func sightingsOfKind(reading, kind string) []string {
	kept := []string{}

	for _, line := range strings.Split(reading, "\n") {
		got, text, found := strings.Cut(line, linkFieldSeparator)
		if found && got == kind {
			kept = append(kept, strconv.Quote(text))
		}
	}

	return kept
}
