package steps

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// wordmarkAnchor is the frame part this clause names in words; every other
	// target it takes is written in the selector grammar.
	wordmarkAnchor = "the wordmark"
	// tokenTrackingFunc answers verdictOK when changing the token at the root
	// repaints the element, and with what it read when it does not. Transitions are
	// stopped: an animated repaint reads the OLD value for a frame, a pass nobody saw.
	tokenTrackingFunc = `((el, token) => {
	const root = document.documentElement
	const declared = getComputedStyle(root).getPropertyValue(token).trim()
	if (declared === "") { return "the page defines no " + token }
	const stop = document.createElement("style")
	stop.textContent = "*, *::before, *::after " +
		"{ transition: none !important; animation: none !important }"
	document.head.appendChild(stop)
	const dump = () => {
		const style = getComputedStyle(el)
		let out = ""
		for (let index = 0; index < style.length; index++) {
			out += style[index] + ":" + style.getPropertyValue(style[index]) + ";"
		}
		return out
	}
	const length = /^(-?[0-9.]+)(px|rem|em|%|vh|vw|ch)$/.exec(declared)
	const probe = length ? (parseFloat(length[1]) + 17) + length[2] : "rgb(1, 2, 3)"
	const before = dump()
	const previous = root.style.getPropertyValue(token)
	root.style.setProperty(token, probe)
	const after = dump()
	if (previous === "") { root.style.removeProperty(token) }
	else { root.style.setProperty(token, previous) }
	stop.remove()
	if (before !== after) { return "ok" }
	return "its computed style is unchanged after " + token + " was set to " +
		probe + ", so it is not painted from that token"
})`
)

// registerTokenTrackingSteps binds the clause holding an element to being painted
// FROM a token rather than merely agreeing with one: the token is changed at the
// root and the element must move with it.
func registerTokenTrackingSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+wordmarkAnchor+`|`+selectorPattern+`) tracks the token `+
		`"([^"]*)" when it is changed$`, assertTracksToken)
}

// assertTracksToken changes the token at the root, reads whether the element's
// computed style moved with it, and puts the token back.
func assertTracksToken(state *State, args []string) error {
	name, locator, err := trackedElement(state, args[0])
	if err != nil {
		return err
	}

	token := args[1]

	got, matched, err := await(readTokenTracking(locator, token), equals(verdictOK))
	if err != nil {
		return state.fail("%s: %w", name, err)
	}

	if !matched {
		return state.fail("%s does not track %s: %s", name, token, got)
	}

	return nil
}

// trackedElement resolves what the clause named: the wordmark, held to a testid
// here, or any element the selector grammar reaches.
func trackedElement(state *State, text string) (string, playwright.Locator, error) {
	if text == wordmarkAnchor {
		locator, err := locateFrame(state, wordmarkTestID, wordmarkAnchor)

		return wordmarkAnchor, locator, err
	}

	sel, locator, err := locateStep(state, text)

	return sel.String(), locator, err
}

// readTokenTracking reads that verdict as a reader, so the clause polls through
// the same await every value clause uses.
func readTokenTracking(locator playwright.Locator, token string) func() (string, error) {
	return readProbe(locator, fmt.Sprintf(`el => %s(el, %q)`, tokenTrackingFunc, token))
}
