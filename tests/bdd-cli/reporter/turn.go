package main

import "time"

// TurnStatus is how an AI turn ended.
type TurnStatus string

const (
	// TurnOpen is a turn with a dispatch record and no completion
	// record. Either the process was killed mid-turn or it is still
	// running; the log cannot tell those apart, so neither is claimed.
	TurnOpen TurnStatus = "open"
	// TurnOK is a turn that returned normally.
	TurnOK TurnStatus = "ok"
	// TurnFailed is a turn whose provider returned an error.
	TurnFailed TurnStatus = "failed"
)

// Turn is one call the engine made to a model CLI, folded from the three
// records it leaves in the engine log: the dispatch, the usage report
// (claude only), and the completion.
type Turn struct {
	Number int
	Role   string
	CLI    string
	Model  string

	// Started and Ended bracket the whole provider call.
	Started time.Time
	Ended   time.Time
	// FirstOutput is the first AssistantMessage — the moment the CLI
	// finished booting and the model started producing. ResultAt is the
	// terminal message. Both are zero for CLIs that stream nothing,
	// which is what makes their turns unsplittable.
	FirstOutput time.Time
	ResultAt    time.Time

	Duration time.Duration
	// DurationIsFloor marks a duration derived from the last log record
	// rather than from the router's own measurement — a lower bound.
	DurationIsFloor bool

	Status TurnStatus
	Error  string

	CostUSD float64
	HasCost bool
	// Tokens keeps the four counters apart rather than pre-summing:
	// cache reads and cache writes are priced differently from ordinary
	// input, so a turn that looks expensive per token may just be one
	// that re-uploaded its system prompt.
	Tokens TokenCounts

	// SystemLength and UserLength are the prompt sizes the dispatch
	// record carries, available even when the artifacts are gone.
	SystemLength int
	UserLength   int

	AllowedTools    []string
	DisallowedTools []string
	ToolCalls       []ToolCall

	Invocation Invocation
	Cell       checklistCell
	// Inputs are the artifacts written before the turn was dispatched,
	// Outputs the ones written after it completed.
	Inputs  []Artifact
	Outputs []Artifact
}

// TokenCounts is one turn's usage, by kind.
type TokenCounts struct {
	Input         int
	Output        int
	CacheRead     int
	CacheCreation int
}

// Total is every counter summed — what the index tables report.
func (t *TokenCounts) Total() int {
	return t.Input + t.Output + t.CacheRead + t.CacheCreation
}

// Add folds one logged counter into the set.
func (t *TokenCounts) Add(key string, count int) {
	switch key {
	case "input_tokens":
		t.Input += count
	case "output_tokens":
		t.Output += count
	case "cache_read_input_tokens":
		t.CacheRead += count
	case "cache_creation_input_tokens":
		t.CacheCreation += count
	}
}

// BootSplit is a turn broken into CLI boot, generation and teardown.
// Present only for a provider that streams its session messages.
type BootSplit struct {
	Boot       time.Duration
	Generation time.Duration
	Teardown   time.Duration
	Known      bool
}

// Split divides the turn into the three spans its message stream
// exposes. Known is false when the CLI streamed nothing.
func (t *Turn) Split() BootSplit {
	if t.FirstOutput.IsZero() || t.Started.IsZero() {
		return BootSplit{}
	}

	split := BootSplit{Boot: t.FirstOutput.Sub(t.Started), Known: true}

	if !t.ResultAt.IsZero() {
		split.Generation = t.ResultAt.Sub(t.FirstOutput)

		if !t.Ended.IsZero() {
			split.Teardown = t.Ended.Sub(t.ResultAt)
		}
	}

	return split
}

// Seconds is the turn's duration as a float, for the arithmetic the
// report does on it.
func (t *Turn) Seconds() float64 {
	return t.Duration.Seconds()
}
