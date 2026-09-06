package steps

import (
	"encoding/json"
	"errors"

	"github.com/playwright-community/playwright-go"
)

// ErrNoBoxSnapshot is returned when a clause compares an element to how it was
// and no earlier step measured the page.
var ErrNoBoxSnapshot = errors.New("no earlier step measured the page")

// boxesProbe reads every testid's box in ONE evaluation: a clause about the
// layout is about the page as it stood at one instant, not element by element.
const boxesProbe = `() => JSON.stringify(Object.fromEntries(
  Array.from(document.querySelectorAll("[data-testid]")).map(el => {
    const box = el.getBoundingClientRect()
    return [el.getAttribute("data-testid"),
      {x: box.x, y: box.y, width: box.width, height: box.height}]
  })))`

// elementBox is one element's rectangle in CSS pixels, as the page laid it out.
type elementBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// rememberPageState records what a "…than it was" clause is compared against and
// where the page's request log then stood. Every When that changes the layout
// calls it BEFORE acting; a page it cannot measure records nothing and says so.
func rememberPageState(state *State) {
	if state.Page == nil {
		return
	}

	state.RequestsBefore = requestCount(state)

	raw, err := probeString(state.Page, boxesProbe)
	if err != nil {
		return
	}

	boxes := map[string]elementBox{}

	err = json.Unmarshal([]byte(raw), &boxes)
	if err != nil {
		return
	}

	state.BoxesBefore = boxes
}

// snapshotBox is one element's box as the last snapshot found it. A keyed or
// nested reference is refused: the snapshot holds one box per testid.
func snapshotBox(state *State, sel selector) (elementBox, error) {
	if sel.Key != "" || sel.Child != "" {
		return elementBox{}, state.fail("%w: %s is not a plain testid",
			ErrNoBoxSnapshot, sel)
	}

	if state.BoxesBefore == nil {
		return elementBox{}, state.fail("%w, so there is nothing to compare %s to",
			ErrNoBoxSnapshot, sel)
	}

	box, measured := state.BoxesBefore[sel.Name]
	if !measured {
		return elementBox{}, state.fail("%w: it showed no %s", ErrNoBoxSnapshot, sel)
	}

	return box, nil
}

// currentBox measures one element now, through the same selector grammar every
// other clause reads.
func currentBox(state *State, text string) (selector, elementBox, error) {
	sel, locator, err := locateStep(state, text)
	if err != nil {
		return selector{}, elementBox{}, err
	}

	box, err := locator.BoundingBox()
	if err != nil {
		return sel, elementBox{}, state.fail("measuring %s: %w", sel, err)
	}

	if box == nil {
		return sel, elementBox{}, state.fail("%s: %w", sel, ErrNoBoundingBox)
	}

	return sel, boxOf(box), nil
}

// boxOf is a playwright rectangle as this suite keeps it.
func boxOf(box *playwright.Rect) elementBox {
	return elementBox{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height}
}
