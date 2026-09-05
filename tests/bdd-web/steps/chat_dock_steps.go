package steps

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoChatMessage is returned when a clause is about a chat turn and no When
// sent one.
var ErrNoChatMessage = errors.New("no step sent a chat message")

// ErrNoChatMarker is returned when a clause is about the marker a message
// carried and no When sent a marked one.
var ErrNoChatMarker = errors.New("no step sent a uniquely-marked chat message")

const (
	// The dock's selector contract, as the registry writes it.
	chatToggleTestID  = "chat-dock-toggle"
	chatPanelTestID   = "chat-dock-panel"
	chatHistoryTestID = "chat-dock-history"
	chatInputTestID   = "chat-dock-input"
	chatSendTestID    = "chat-dock-send"
	// chatDriverSetting switches the CLI onto its scripted chat driver, so a
	// chat scenario grades the workspace rather than a model's wording
	// (services/bdd-cli/internal/app/remote/chat.go).
	chatDriverSetting = "TRUE_BDD_CHAT_DRIVER=deterministic"
	// chatReplyTimeout caps waiting on a chat turn: it crosses the relay to the
	// CLI and back, which outlasts a render.
	chatReplyTimeout = 60 * time.Second
	// dragSteps spreads a drag over several pointer moves — a divider listening
	// for mousemove never sees a single jump — and centreFraction places the
	// press in the middle of the control it grabs.
	dragSteps      = 10
	centreFraction = 0.5
)

// registerChatDockSteps binds the docked chat's vocabulary: the connection its
// scripted driver needs, opening it, the messages a scenario sends, and what
// the conversation and the documents then say.
func registerChatDockSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the workspace is connected with the deterministic chat driver$`,
		connectWorkspaceWithChatDriver)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) has opened the chat$`,
		openChat)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) sends a `+
		`uniquely-marked chat message$`, sendMarkedChatMessage)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) asks the chat to `+
		`add a uniquely-named (term|scenario)$`, askChatToAdd)
	suite.Step(`^the workspace is connected with a real model driving the chat$`,
		connectWorkspaceWithRealChat)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) asks the chat in `+
		`prose to add a uniquely-named (term|scenario)$`, askChatInProseToAdd)
	suite.Step(`^the (Product Owner|System Architect|Quality Engineer) drags `+
		`(`+selectorPattern+`) (\d+) pixels (left|right)$`, dragElement)
	suite.Step(`^(`+selectorPattern+`) contains the chat marker$`, assertContainsChatMarker)
	suite.Step(`^the chat answered$`, assertChatAnswered)
	suite.Step(`^"([^"]+)" holds the new scenario$`, assertFileHoldsNewScenario)
	suite.Step(`^the page shows the new term's ([a-z][a-z0-9-]*)$`, assertNewTermShown)
}

// connectWorkspaceWithChatDriver is the workspace connection with the CLI's
// scripted chat driver switched on.
func connectWorkspaceWithChatDriver(state *State, _ []string) error {
	return connectWorkspaceWith(state, chatDriverSetting)
}

// connectWorkspaceWithRealChat leaves that driver OFF, so the chat is answered
// by the model the CLI ships with — which is the whole subject of a scenario
// about a real turn.
func connectWorkspaceWithRealChat(state *State, _ []string) error {
	return connectWorkspaceWith(state)
}

// openChat is the Given a chat scenario opens with: the toggle clicked only if
// the panel is not already showing, and held to having opened — an unverified
// precondition is not a precondition. args[0] is the role, discarded.
func openChat(state *State, _ []string) error {
	shown, err := elementShown(state, chatPanelTestID)
	if err != nil {
		return err
	}

	if !shown {
		err = clickElement(state, []string{"", chatToggleTestID})
		if err != nil {
			return err
		}
	}

	_, _, err = locateStep(state, chatPanelTestID)

	return err
}

// sendMarkedChatMessage sends a marker no other run can have produced, and
// waits for the dock to hold it: a message the page never took is not sent.
func sendMarkedChatMessage(state *State, _ []string) error {
	marker := uniqueName(state, "chat")

	err := sendChatMessage(state, marker)
	if err != nil {
		return err
	}

	state.ChatMarker = marker

	return assertContainsChatMarker(state, []string{chatHistoryTestID})
}

// askChatToAdd sends the scripted directive the CLI's deterministic driver
// answers with a schema-aware edit, naming a term or a scenario no other run
// can have produced. args[0] is the role, discarded.
func askChatToAdd(state *State, args []string) error {
	name, err := coinChatTarget(state, args[1])
	if err != nil {
		return err
	}

	return sendChatMessage(state, fmt.Sprintf("@probe add-%s %s", args[1], name))
}

// askChatInProseToAdd asks in the reader's own words instead of the driver's
// directive: with a real model answering, deciding WHICH document to edit is
// the model's work, and that is what the clause saying "in prose" is for.
func askChatInProseToAdd(state *State, args []string) error {
	kind := args[1]

	name, err := coinChatTarget(state, kind)
	if err != nil {
		return err
	}

	return sendChatMessage(state, fmt.Sprintf(
		"Please add a %s named %s to the document I have open, and save the change.",
		kind, name))
}

// coinChatTarget snapshots the writable documents, coins a name no other run can
// have produced and files it under the kind the clause named, so both chat
// clauses grade the same marker.
func coinChatTarget(state *State, kind string) (string, error) {
	err := rememberAllowedDocuments(state)
	if err != nil {
		return "", err
	}

	name := uniqueName(state, kind)

	if kind == "scenario" {
		name = strings.ToUpper(name)
		state.NewScenarioID = name

		return name, nil
	}

	state.NewTerm = name

	return name, nil
}

// sendChatMessage types one message into the dock and sends it, keeping what
// the history held first: the answered clause has no other evidence.
func sendChatMessage(state *State, text string) error {
	before, err := chatHistoryText(state)
	if err != nil {
		return err
	}

	_, input, err := locateStep(state, chatInputTestID)
	if err != nil {
		return err
	}

	err = input.Fill(text)
	if err != nil {
		return state.fail("typing %q into %s: %w", text, chatInputTestID, err)
	}

	_, send, err := locateStep(state, chatSendTestID)
	if err != nil {
		return err
	}

	err = send.Click()
	if err != nil {
		return state.fail("clicking %s: %w", chatSendTestID, err)
	}

	state.ChatHistoryBefore, state.ChatSentText, state.ChatSent = before, text, true

	return nil
}

// chatHistoryText is what the dock's history reads now.
func chatHistoryText(state *State) (string, error) {
	_, history, err := locateStep(state, chatHistoryTestID)
	if err != nil {
		return "", err
	}

	text, err := readInnerText(history)()
	if err != nil {
		return "", state.fail("%s: %w", chatHistoryTestID, err)
	}

	return text, nil
}

// dragElement drags one control by the distance the step names, taking the
// layout snapshot the "…than it was" clause after it is compared against.
// args[0] is the role, discarded.
func dragElement(state *State, args []string) error {
	rememberPageState(state)

	sel, locator, err := locateStep(state, args[1])
	if err != nil {
		return err
	}

	distance, err := pixels(state, args[2])
	if err != nil {
		return err
	}

	if args[3] == "left" {
		distance = -distance
	}

	return dragBy(state, sel, locator, distance)
}

// dragBy presses the element's centre, moves the pointer horizontally in steps
// and releases.
func dragBy(state *State, sel selector, locator playwright.Locator,
	distance float64,
) error {
	box, err := locator.BoundingBox()
	if err != nil {
		return state.fail("measuring %s: %w", sel, err)
	}

	if box == nil {
		return state.fail("%s: %w", sel, ErrNoBoundingBox)
	}

	page, err := state.page()
	if err != nil {
		return err
	}

	fromX := box.X + box.Width*centreFraction
	fromY := box.Y + box.Height*centreFraction

	err = pressMoveRelease(page.Mouse(), fromX, fromY, distance)
	if err != nil {
		return state.fail("dragging %s by %.0f pixels: %w", sel, distance, err)
	}

	return nil
}

// pressMoveRelease is the pointer gesture itself, kept apart so the failure
// above names the control rather than the primitive that refused.
func pressMoveRelease(mouse playwright.Mouse, fromX, fromY, distance float64) error {
	err := mouse.Move(fromX, fromY)
	if err != nil {
		return fmt.Errorf("move the pointer onto the control: %w", err)
	}

	err = mouse.Down()
	if err != nil {
		return fmt.Errorf("press the pointer: %w", err)
	}

	err = mouse.Move(fromX+distance, fromY,
		playwright.MouseMoveOptions{Steps: playwright.Int(dragSteps)})
	if err != nil {
		return fmt.Errorf("drag the pointer: %w", err)
	}

	err = mouse.Up()
	if err != nil {
		return fmt.Errorf("release the pointer: %w", err)
	}

	return nil
}

// assertContainsChatMarker holds an element's text to carrying the marker the
// When sent, which is the containment clause under a different name.
func assertContainsChatMarker(state *State, args []string) error {
	if state.ChatMarker == "" {
		return state.fail("%w", ErrNoChatMarker)
	}

	return assertElementContainsText(state, []string{args[0], state.ChatMarker})
}

// assertChatAnswered holds the dock to having said something back: the history
// grew past what it held plus the message that was sent into it.
func assertChatAnswered(state *State, _ []string) error {
	if !state.ChatSent {
		return state.fail("%w", ErrNoChatMessage)
	}

	_, history, err := locateStep(state, chatHistoryTestID)
	if err != nil {
		return err
	}

	floor := len(state.ChatHistoryBefore) + len(state.ChatSentText)

	got, matched, err := awaitWithin(chatReplyTimeout, readInnerText(history),
		func(value string) bool { return len(value) > floor })
	if err != nil {
		return state.fail("%s: %w", chatHistoryTestID, err)
	}

	if !matched {
		return state.fail("%s reads %q, which is no longer than what it held with the "+
			"sent message added to it — want a reply beside it", chatHistoryTestID, got)
	}

	return nil
}

// assertFileHoldsNewScenario holds the registry on disk to declaring the
// scenario the chat added. Polled on the chat's budget: the save is the CLI's
// work and lands after the browser is answered.
func assertFileHoldsNewScenario(state *State, args []string) error {
	if state.NewScenarioID == "" {
		return state.fail("%w", ErrNoNewScenario)
	}

	relPath := args[0]

	got, matched, err := awaitWithin(chatReplyTimeout,
		readNodeBlock(state, relPath, scenariosNode),
		func(value string) bool { return strings.Contains(value, state.NewScenarioID) })
	if err != nil {
		return state.fail("reading the %q node of %s: %w", scenariosNode, relPath, err)
	}

	if !matched {
		return state.fail("the %q node of %s does not hold %q; it reads:\n%s",
			scenariosNode, relPath, state.NewScenarioID, got)
	}

	state.SavedPath = relPath

	return nil
}

// assertNewTermShown holds the derived view to showing a row for the term the
// chat added, found by the name it alone carries.
func assertNewTermShown(state *State, args []string) error {
	if state.NewTerm == "" {
		return state.fail("%w", ErrNoNewTerm)
	}

	return assertRowCarrying(state, args[0], state.NewTerm)
}

// assertRowCarrying waits for ONE of a testid family to carry the text, which
// the strict single-element grammar cannot express: a derived view renders a
// row per entry.
func assertRowCarrying(state *State, testID, want string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	script := fmt.Sprintf(
		`() => String(Array.from(document.querySelectorAll('[data-testid=%q]'))
			.filter(el => (el.innerText || "").includes(%q)).length)`, testID, want)

	got, matched, err := awaitWithin(chatReplyTimeout,
		func() (string, error) { return probeString(page, script) },
		func(value string) bool { return value != "0" })
	if err != nil {
		return state.fail("reading the %s elements: %w", testID, err)
	}

	if !matched {
		return state.fail("no %s carries %q (%s are shown)", testID, want, got)
	}

	return nil
}
