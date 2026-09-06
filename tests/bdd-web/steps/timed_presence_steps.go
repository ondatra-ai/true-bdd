package steps

import (
	"strconv"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// millisecondsPerSecond turns a clause's seconds into the milliseconds
// playwright's own waits are written in.
const millisecondsPerSecond = 1000

// registerTimedPresenceSteps binds the two clauses that put a deadline on what
// the page shows: one that must appear inside a window, and one that must not
// appear for the whole of it.
func registerTimedPresenceSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the page shows (`+selectorPattern+`) within (\d+) seconds$`,
		assertShownWithin)
	suite.Step(`^the page shows exactly (\d+) (`+selectorPattern+`) `+
		`for (?:the first )?(\d+) seconds$`, assertCountHeldFor)
}

// assertShownWithin holds the page to showing the element inside the clause's own
// window: a notice that arrives after it is a notice the reader was not given.
func assertShownWithin(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	budget, err := stepSeconds(state, args[1])
	if err != nil {
		return err
	}

	err = sel.element(page).WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(budget * millisecondsPerSecond),
	})
	if err != nil {
		return state.fail("the page never showed %s within %s seconds: %w\n%s",
			sel, args[1], err, visibleText(page))
	}

	return nil
}

// assertCountHeldFor holds the count for the WHOLE window rather than reading it
// once: the clause is about a notice that must not flash, and one reading cannot
// see a flash.
func assertCountHeldFor(state *State, args []string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	sel, err := parseSelector(args[1])
	if err != nil {
		return state.fail("%w", err)
	}

	budget, err := stepSeconds(state, args[2])
	if err != nil {
		return err
	}

	return holdCountFor(state, readCount(page, sel), sel, args[0],
		time.Duration(budget)*time.Second)
}

// holdCountFor re-reads until the window is out, failing at the first reading
// that is not the one the clause named and saying how far in it came.
func holdCountFor(state *State, read func() (string, error), sel selector,
	want string, window time.Duration,
) error {
	start := time.Now()
	deadline := start.Add(window)

	for {
		got, err := read()
		if err != nil {
			return state.fail("%s: %w", sel, err)
		}

		if got != want {
			return state.fail("the page shows %s %s after %s, want exactly %s "+
				"for the whole %s", got, sel, time.Since(start), want, window)
		}

		if !time.Now().Before(deadline) {
			return nil
		}

		time.Sleep(valuePollInterval)
	}
}

// stepSeconds parses a clause's second count, which its own \d+ capture
// guarantees.
func stepSeconds(state *State, text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, state.fail("the step's second count %q does not parse: %w", text, err)
	}

	return value, nil
}
