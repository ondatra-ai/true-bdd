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
// is looking at. A context per scenario, not a page per suite — sharing
// one would leak cookies/storage between scenarios, the classic browser-suite flake.
type State struct {
	T        *testing.T
	Scenario bddgo.Scenario
	Harness  *Harness

	Context playwright.BrowserContext
	Page    playwright.Page

	// Tree is the host project a Given step materialized.
	Tree *ProjectTree
	// Remote is the `true-bdd remote` a Given step started in it.
	Remote *Remote
	// Session is the registry entry that remote registered, resolved by
	// the Then step that waits for the relay to list it.
	Session *sessionSummary
	// Runs maps the label a scenario dispatched a run under ("R1") to the
	// run id the relay created, which a later clause names as {R1}.
	Runs map[string]string
	// Dispatches holds the body each labelled dispatch sent, so the clause
	// about re-sending the same token sends exactly that body again.
	Dispatches map[string]dispatchBody
	// Response is the last relay response an API step read, so the clause
	// after it reads the error name off the same body.
	Response *apiResponse
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
			T:          world.T,
			Scenario:   world.Scenario,
			Harness:    harness,
			Context:    browserContext,
			Runs:       map[string]string{},
			Dispatches: map[string]dispatchBody{},
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

// fail prefixes a step failure with the scenario id via format-string splicing, preserving the caller's `%w`.
//
//nolint:err113 // the message IS the failure; callers pass %w wherever a sentinel exists.
func (s *State) fail(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.Scenario.ID}, args...)...)
}
