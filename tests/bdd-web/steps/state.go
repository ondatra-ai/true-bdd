package steps

import (
	"errors"
	"fmt"
	"testing"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
)

// ErrNoPage is returned when a step needs a browser page and no earlier
// step opened one.
var ErrNoPage = errors.New("no step has opened a page yet")

// State is one scenario's world: its own browser context and the page it
// is looking at.
//
// A context per scenario, not a page per suite: cookies, storage and
// service workers all live on the context, and two scenarios sharing one
// would make the second's result depend on the first's — which is the
// class of flake a browser suite is most often actually suffering from.
type State struct {
	T        *testing.T
	Scenario bddgo.Scenario
	Harness  *Harness

	Context playwright.BrowserContext
	Page    playwright.Page
}

// NewState returns the per-scenario state constructor bddgo calls before
// the first step. The browser context is opened here and closed by a
// cleanup, so a scenario that fails mid-way still gives its context back.
func NewState(harness *Harness) func(*bddgo.World) (*State, error) {
	return func(world *bddgo.World) (*State, error) {
		browserContext, err := harness.Browser.NewContext()
		if err != nil {
			return nil, fmt.Errorf("open a browser context: %w", err)
		}

		world.T.Cleanup(func() { _ = browserContext.Close() })

		return &State{
			T:        world.T,
			Scenario: world.Scenario,
			Harness:  harness,
			Context:  browserContext,
		}, nil
	}
}

// page returns the scenario's page, or says which step should have
// opened it.
func (s *State) page() (playwright.Page, error) {
	if s.Page == nil {
		return nil, s.fail("%w", ErrNoPage)
	}

	return s.Page, nil
}

// fail prefixes a step failure with the scenario id, so a failure read
// out of a long log says which scenario produced it without scrolling
// up. The caller's own `%w` survives: the id is spliced into the format
// string rather than wrapped around a second Errorf.
//
//nolint:err113 // the message IS the failure; callers pass %w wherever a sentinel exists.
func (s *State) fail(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.Scenario.ID}, args...)...)
}
