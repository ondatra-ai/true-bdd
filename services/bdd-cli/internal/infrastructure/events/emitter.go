// Package events implements the child-local structured event channel
// (plan §3.2). When TRUE_BDD_EVENTS_FILE names a path, the running
// command appends JSONL events to it — a `prompt` event immediately
// before it blocks on stdin, and a `result` event after finalization.
// The `true-bdd remote` parent tails this file and translates the
// child-local events into its own server-owned (run_id, seq) stream.
//
// When the variable is unset every emit is a no-op, so the default CLI
// behavior is byte-for-byte unchanged. Emission is fail-closed: if the
// append cannot be written, a prompt could otherwise block invisibly,
// so the emitter prints a stderr sentinel and exits non-zero.
package events

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// EventsFileEnv is the environment variable naming the JSONL event
// channel file. Unset ⇒ emission disabled.
const EventsFileEnv = "TRUE_BDD_EVENTS_FILE"

// FailClosedSentinel is printed to stderr right before a failed append
// exits the process non-zero, so the remote's error(no_result) exit
// classification has a legible cause in the captured log.
const FailClosedSentinel = "true-bdd: FATAL event-channel append failed"

const eventFilePerm = 0o600

// Kind enumerates the prompt kinds the fix loop can publish.
type Kind string

const (
	// KindChoice is the apply / refine / exit fix-action prompt.
	KindChoice Kind = "choice"
	// KindClarify is a single-answer clarification question (optionally
	// carrying numbered options).
	KindClarify Kind = "clarify"
	// KindFreetext is the multiline refinement-feedback prompt.
	KindFreetext Kind = "freetext"
)

// Emitter appends structured events to the event-channel file. Its ordinal
// and prompt counters are monotonic per instance, so ids stay distinct and
// the result ordinal always follows the prompt ordinals (plan §3.2, finding 7).
type Emitter struct {
	mu      sync.Mutex
	path    string
	ordinal int
	prompts int
}

// sharedMu guards the process-wide emitter's single ordinal source.
//
//nolint:gochecknoglobals // the child-local ordinal source is process-wide
var (
	sharedMu sync.Mutex
	shared   *Emitter
)

// NewEmitter returns the process-wide emitter bound to TRUE_BDD_EVENTS_FILE,
// keyed on that path so a test repointing the env var gets a fresh instance;
// an empty path (variable unset) yields a no-op emitter.
func NewEmitter() *Emitter {
	path := os.Getenv(EventsFileEnv)

	sharedMu.Lock()
	defer sharedMu.Unlock()

	if shared == nil || shared.path != path {
		shared = &Emitter{path: path}
	}

	return shared
}

// Enabled reports whether the event channel is active.
func (e *Emitter) Enabled() bool {
	return e.path != ""
}

// EmitChoicePrompt writes the apply/refine/exit choice prompt event.
// Call it immediately before blocking on stdin for the user's choice.
func (e *Emitter) EmitChoicePrompt(actions []string) {
	e.emitPrompt(KindChoice, map[string]any{"actions": actions})
}

// EmitClarifyPrompt writes a clarify prompt event carrying the question
// text and any numbered options (the browser answers by option number).
func (e *Emitter) EmitClarifyPrompt(question string, options []string) {
	e.emitPrompt(KindClarify, map[string]any{
		"question": question,
		"options":  options,
	})
}

// EmitFreetextPrompt writes the multiline refinement-feedback prompt
// event. Call it immediately before reading the multiline block.
func (e *Emitter) EmitFreetextPrompt(prompt string) {
	e.emitPrompt(KindFreetext, map[string]any{"prompt": prompt})
}

// EmitResult writes the terminal result event after finalization: outcome
// is the stop-reason classification, and finalizationOK is false when the
// post-walk write failed (plan §3.2).
func (e *Emitter) EmitResult(outcome string, finalizationOK bool, detail string) {
	if e.path == "" {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.ordinal++

	e.append(map[string]any{
		"type":            "result",
		"ordinal":         e.ordinal,
		"outcome":         outcome,
		"finalization_ok": finalizationOK,
		"detail":          detail,
	})
}

func (e *Emitter) emitPrompt(kind Kind, payload map[string]any) {
	if e.path == "" {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.ordinal++
	e.prompts++

	e.append(map[string]any{
		"type":      "prompt",
		"prompt_id": fmt.Sprintf("prompt-%d", e.prompts),
		"kind":      string(kind),
		"ordinal":   e.ordinal,
		"payload":   payload,
	})
}

// append serializes one event as a JSON line and appends it. Any
// failure is fatal (fail-closed): the caller may be about to block on a
// prompt whose event never reached the remote.
func (e *Emitter) append(event map[string]any) {
	line, err := json.Marshal(event)
	if err != nil {
		e.fail(err)

		return
	}

	file, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, eventFilePerm)
	if err != nil {
		e.fail(err)

		return
	}
	defer func() { _ = file.Close() }()

	_, err = file.Write(append(line, '\n'))
	if err != nil {
		e.fail(err)
	}
}

// fail prints the sentinel and exits non-zero. A telemetry append that
// cannot land must never let a prompt block invisibly.
func (e *Emitter) fail(err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", FailClosedSentinel, err)
	os.Exit(1)
}
