package steps

import (
	"fmt"
	"strconv"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

const (
	// shiftSettleMs is how long the reading waits after subscribing, so the
	// entries the observer replays have arrived before it answers.
	shiftSettleMs = 300
	// wholeDocumentScope is the scope a clause about the whole page names: every
	// shift counts, whatever moved.
	wholeDocumentScope = ""
)

// layoutShiftFunc answers verdictOK when no non-input shift landed inside the
// window — and, when a scope is named, none moved a node inside it. Buffered:
// a layout-shift entry is observable only through a PerformanceObserver.
const layoutShiftFunc = `((windowMs, scopeCSS, ok, limit, settleMs) => new Promise(resolve => {
	const offenders = []
	const elementOf = source => {
		const node = source && source.node
		if (!node) { return null }
		return node.nodeType === 1 ? node : node.parentElement
	}
	const nameOf = source => {
		const el = elementOf(source)
		if (!el) { return "an unnamed node" }
		const named = el.closest("[data-testid]")
		return named ? named.getAttribute("data-testid") : el.tagName.toLowerCase()
	}
	const observer = new PerformanceObserver(list => {
		for (const entry of list.getEntries()) {
			if (entry.hadRecentInput || !(entry.value > 0)) { continue }
			if (windowMs > 0 && entry.startTime > windowMs) { continue }
			const sources = Array.from(entry.sources || [])
			if (scopeCSS !== "" && !sources.some(source => {
				const el = elementOf(source)
				return el && el.closest(scopeCSS)
			})) { continue }
			const names = sources.map(nameOf).join(" + ")
			offenders.push((names || "an unnamed node") + " shifted by " +
				entry.value.toFixed(4) + " at " + Math.round(entry.startTime) + "ms")
		}
	})
	observer.observe({type: "layout-shift", buffered: true})
	setTimeout(() => {
		observer.disconnect()
		resolve(offenders.length === 0 ? ok : offenders.slice(0, limit).join("; "))
	}, Math.max(settleMs, windowMs - performance.now()))
}))`

// registerLayoutShiftSteps binds the clauses about the page settling without
// moving: inside a window the clause names, and around a region it names.
func registerLayoutShiftSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^no non-input layout shift occurred in the first (\d+) seconds$`,
		assertNoShiftWithin)
	suite.Step(`^no non-input layout shift moved any ([a-z][a-z0-9-]*) element$`,
		assertNoShiftInRegion)
}

// assertNoShiftWithin holds the load to settling without moving inside the
// window the clause names; the probe itself waits that window out.
func assertNoShiftWithin(state *State, args []string) error {
	seconds, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return state.fail("the step's second count %q does not parse: %w", args[0], err)
	}

	return assertNoLayoutShift(state, seconds*millisecondsPerSecond, wholeDocumentScope,
		fmt.Sprintf("no non-input layout shift may occur in the first %s seconds", args[0]))
}

// assertNoShiftInRegion holds one region of the page to not moving, however
// late the data that fills it arrives.
func assertNoShiftInRegion(state *State, args []string) error {
	region := args[0]

	return assertNoLayoutShift(state, 0, fmt.Sprintf(`[data-testid^=%q]`, region),
		fmt.Sprintf("no non-input layout shift may move any %s element", region))
}

// assertNoLayoutShift reads that verdict ONCE rather than polling: a shift the
// page already recorded does not un-happen, and the probe waits its own window.
func assertNoLayoutShift(state *State, windowMs float64, scopeCSS, wanted string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	script := fmt.Sprintf(`() => %s(%g, %q, %q, %d, %d)`,
		layoutShiftFunc, windowMs, scopeCSS, verdictOK, offenderLimit, shiftSettleMs)

	got, err := probeString(page, script)
	if err != nil {
		return state.fail("reading whether %s: %w", wanted, err)
	}

	if got != verdictOK {
		return state.fail("%s, but %s", wanted, got)
	}

	return nil
}
