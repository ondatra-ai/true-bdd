package steps

import (
	"errors"
	"fmt"
	"testing"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
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

	// OpenedURL is the URL the workspace was last opened on, which the clause
	// about the URL not changing is compared against.
	OpenedURL string

	// RelayURL is where the relay this scenario's clauses are about answers:
	// the harness's own, unless a Given started one of the scenario's own.
	RelayURL string
	// SecondRelayURL is the relay a Given started BESIDE that one, which a
	// clause names to hold two differently-configured processes apart.
	SecondRelayURL string

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
	// RunSessions maps that same label to the session the run was dispatched
	// on: a scenario may run several remotes, and a run belongs to one of them.
	RunSessions map[string]string
	// Remotes maps the label a scenario started a remote under ("A") to that
	// remote, and Sessions to the registry entry it registered.
	Remotes  map[string]*Remote
	Sessions map[string]*sessionSummary
	// StoppedSession is the entry of the remote a When stopped, which the
	// clauses about a session that must have vanished are about.
	StoppedSession *sessionSummary
	// ClickedSession is the session whose row a When opened, which "that
	// session's workspace home" names.
	ClickedSession string
	// Dispatches holds the body each labelled dispatch sent, so the clause
	// about re-sending the same token sends exactly that body again.
	Dispatches map[string]dispatchBody
	// Response is the last relay response an API step read, so the clause
	// after it reads the error name off the same body.
	Response *apiResponse
	// Agent is the agent a register step last created, whose correlation the
	// poll and reply bodies after it carry.
	Agent *agentSession
	// Agents maps the label a scenario registered an agent under to it.
	Agents map[string]*agentSession
	// Relay is the relay a Given started for this scenario, kept so the restart
	// clause returns THAT process rather than the shared one.
	Relay *relay
	// SessionBefore is the registry entry as it stood before a When step
	// restarted the relay, which the unchanged clause is compared against.
	SessionBefore *sessionSummary
	// Requests maps the label a scenario sent a call under ("Q1") to the
	// outstanding call, which nothing answers until an agent polls its work.
	Requests map[string]*pendingRequest
	// Works maps the label an agent's poll filed its work item under ("W1").
	Works map[string]*workItem
	// Polls maps an agent's label to the status its last poll answered, so the
	// credentials clause reads what the relay actually said.
	Polls map[string]int
	// PriorAgents maps an agent's label to what its PREVIOUS registration
	// negotiated, so a clause about the superseded epoch sends the correlation
	// a re-register retired rather than an arithmetic guess at it.
	PriorAgents map[string]*agentSession
	// Reregistered is the label of the agent a re-register step last renewed,
	// which the epoch clause reads without the step naming it again.
	Reregistered string
	// Prompts maps a run's label to the prompt id the scenario last saw it
	// blocked on, so "the same prompt" is the one an earlier step observed and
	// not whatever the run happens to hold when it is read again.
	Prompts map[string]string
	// Prompted is the label of the run whose prompt was observed last, which
	// the clauses that say "the same prompt" without naming a run read.
	Prompted string
	// PromptsBeforeRestart is the prompt each run was blocked on when a When
	// restarted the relay — the past-tense clause's only evidence, since the
	// answer sent after the restart overwrites Prompts.
	PromptsBeforeRestart map[string]string
	// ChildBeforeRestart is the command child each prompted run was running in
	// when a When restarted the relay, and ChildAtAnswer the one it was running
	// in when the answer was posted: the clause compares two live readings.
	ChildBeforeRestart map[string]commandChild
	ChildAtAnswer      map[string]commandChild
	// CommandGroups is every process group a Given saw a command child alive in,
	// which the clause about what an interrupt left running is graded on.
	CommandGroups []int
	// Answer is the response the last answer a step sent came back with, kept
	// apart from Response so a later read cannot replace what the answer
	// clause grades.
	Answer *apiResponse
	// PromptHistory is every prompt the scenario answered on a run, in order, so
	// the clause about the shape a fix loop took reads what it actually asked.
	PromptHistory map[string][]promptRecord
	// PromptEvents is the event sequence numbers a run held while it was first
	// blocked, which the clauses about what it recorded afterwards compare against.
	PromptEvents map[string][]int
	// Applies counts how often a scenario took the apply choice on a run — the
	// only evidence a loop that converged actually applied anything.
	Applies map[string]int
	// Reads maps the kind of concurrent read a When step fired ("run") to every
	// reply that came back, so a Then grades the whole batch rather than one.
	Reads map[string][]concurrentRead
	// ReadSession is the session that batch was read under, which "the session
	// it was asked about" names.
	ReadSession string
	// Relays maps the label a Given started a relay under ("A") to that process,
	// so a clause names which of two instances it is about.
	Relays map[string]*relay
	// Outputs maps a run's label to how much output the scenario has already
	// watched it publish, which the "more output" clause is compared against.
	Outputs map[string]int
	// LateReplies maps a work label to the marker the reply this suite sent
	// carries, so a clause about a reply the relay must not have kept looks for
	// exactly those bytes.
	LateReplies map[string]string
	// PreSweep maps a session's label to what a further read answered BEFORE the
	// sweep — the past-tense clause's only evidence, since the sweep destroys it.
	PreSweep map[string]int
	// SeededToken is the marker a Given wrote into SeededPath, which the clause
	// about the file's own content looks for in what the page rendered.
	SeededToken string
	// SeededPath is the document that marker was seeded into.
	SeededPath string
	// EditorOriginal is the buffer as the editor first held it, which the restore
	// clause types back.
	EditorOriginal string
	// AppendedComment is the comment the single-document edit typed, which the
	// clause about that document holding it is compared against.
	AppendedComment string
	// NewEndpointPath is the path the endpoint a When declared carries, so the
	// view clause and the file clause look for the same one.
	NewEndpointPath string
	// NewTerm is the uniquely-named term a When typed into the editor, which the
	// document clause and the rendered clause both look for.
	NewTerm string
	// NewServiceName is the uniquely-named service a When declared.
	NewServiceName string
	// SavedPath is the document the last content clause was about, which "that
	// path" in the receipt clause names.
	SavedPath string
	// DocsBefore is every writable document as it stood before this scenario's
	// first edit — the unchanged clause's only evidence.
	DocsBefore map[string]string
	// TreeDocsBefore is those same documents as the tree's Given materialized
	// them, kept apart from DocsBefore so a fix run's clauses read a snapshot no
	// form step took: a run rewrites the registry in place and leaves no other.
	TreeDocsBefore map[string]string
	// SeededTokens maps a product document to the marker seeded into THAT
	// document, so a page rendering another file cannot pass the token clause.
	SeededTokens map[string]string
	// ProductDocuments are the documents a When opened in turn, which every
	// "every product document" clause is about.
	ProductDocuments []productDocument
	// EditedDocuments are the documents a When typed a comment into, each with
	// the comment it typed.
	EditedDocuments []editedDocument
	// NewScenarioID is the id of the scenario a When typed into the registry,
	// which the outline's row clause looks it up by.
	NewScenarioID string
	// SeededFeature is the feature a Given declared in the product's features,
	// which every "the seeded feature" clause is about.
	SeededFeature string
	// NewFeature is the feature a When coined while creating a story, which the
	// features-file clause and the created story's clause both look for.
	NewFeature string
	// BoxesBefore is where every rendered element sat when the last When
	// measured the page — the "…than it was" clauses' only evidence.
	BoxesBefore map[string]elementBox
	// RequestsBefore is how much of the page's request log that When had already
	// seen, so a clause reads only the traffic it caused.
	RequestsBefore int
	// ChatMarker is the marker the last marked chat message carried, which the
	// clause about the conversation following the reader looks for.
	ChatMarker string
	// ChatSentText is that message, and ChatHistoryBefore what the dock held
	// before it was sent: the answered clause is compared against both.
	ChatSentText      string
	ChatHistoryBefore string
	// ChatSent records that a message was sent at all, which an empty history
	// cannot say.
	ChatSent bool
	// SessionsGate is how the sessions read is answered — the page's own traffic
	// until a step holds, fails, hangs or corrupts it, and the mode a page opened
	// after that step starts in.
	SessionsGate string
	// LastSelector is the element the last clause named, which the clauses
	// written about "it" resolve.
	LastSelector string
	// HoveredName is how a clause named the element the last hover pointed at,
	// and HoveredPath the CSS that names it again once the pointer is on it.
	HoveredName string
	HoveredPath string
	// HoveredColour is what it painted its text in BEFORE the pointer arrived,
	// which the clause about a hover changing nothing is compared against.
	HoveredColour string
	// HoverBoxes is where every element sat at that same instant — the "moved by
	// no more than" clauses' only evidence, since a hover leaves nothing behind.
	HoverBoxes map[string]elementBox
	// ChipBoxes is where each inventory chip sat when the list FIRST rendered,
	// keyed by the entry it belongs to.
	ChipBoxes map[string]elementBox
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
			T:                  world.T,
			Scenario:           world.Scenario,
			Harness:            harness,
			Context:            browserContext,
			RelayURL:           harness.BaseURL,
			Runs:               map[string]string{},
			RunSessions:        map[string]string{},
			Remotes:            map[string]*Remote{},
			Sessions:           map[string]*sessionSummary{},
			Dispatches:         map[string]dispatchBody{},
			Agents:             map[string]*agentSession{},
			Requests:           map[string]*pendingRequest{},
			Works:              map[string]*workItem{},
			Polls:              map[string]int{},
			PriorAgents:        map[string]*agentSession{},
			Prompts:            map[string]string{},
			Relays:             map[string]*relay{},
			Outputs:            map[string]int{},
			LateReplies:        map[string]string{},
			PreSweep:           map[string]int{},
			PromptHistory:      map[string][]promptRecord{},
			PromptEvents:       map[string][]int{},
			Applies:            map[string]int{},
			ChildBeforeRestart: map[string]commandChild{},
			ChildAtAnswer:      map[string]commandChild{},
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
