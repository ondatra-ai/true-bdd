package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// ErrNoResult reports a stream that ended without its result envelope.
var ErrNoResult = errors.New("stream ended before the result envelope")

// EventKind names what arrived on a streaming turn.
type EventKind int

const (
	// EventAssistant is one assistant message; Blocks is how many it holds.
	EventAssistant EventKind = iota
	// EventToolUse is one tool-use block within an assistant message.
	EventToolUse
	// EventResult is the envelope that ends the turn.
	EventResult
)

// Event is one record of a streaming turn, handed over AS IT ARRIVES. That is
// the whole point of the streaming form: a caller that only wants the answer
// calls RunJSON, and one that has to timestamp the turn's shape calls this.
type Event struct {
	Kind   EventKind
	Blocks int
	Name   string
	Input  json.RawMessage
}

// RunStream performs one headless turn over stream-json, calling on for every
// record as it arrives, and returns the same Answer RunJSON would.
func RunStream(prompt string, opts Options, on func(Event)) (Answer, error) {
	opts.Stream = true

	sink := &streamSink{on: on}
	stderr := &bytes.Buffer{}

	result, err := shell.Run(context.Background(), append([]string{Bin}, opts.Args(prompt)...), shell.Options{
		Dir:     opts.Dir,
		Env:     opts.Env(),
		Output:  shell.Streams(sink, stderr),
		Timeout: opts.Timeout,
	})
	if err != nil {
		if errors.Is(err, shell.ErrTimeout) {
			return Answer{}, fmt.Errorf("%w after %s", ErrTimeout, opts.Timeout)
		}

		return Answer{}, fmt.Errorf("%w: %w", ErrFailed, err)
	}

	if result.Code != 0 {
		return Answer{}, fmt.Errorf("%w (exit %d): %s",
			ErrFailed, result.Code, truncate(stderr.String(), diagnosticLimit))
	}

	if sink.result == nil {
		return Answer{}, ErrNoResult
	}

	return parseEnvelope(sink.result, opts.Role)
}

// streamSink splits the child's stdout into lines as they land. It is an
// io.Writer rather than a scanner because that is what shell.Streams takes,
// and a pipe nobody drains is how the predecessor deadlocked.
type streamSink struct {
	on      func(Event)
	pending []byte
	result  []byte
}

func (s *streamSink) Write(chunk []byte) (int, error) {
	s.pending = append(s.pending, chunk...)

	for {
		// A chunk boundary is not a line boundary: what is left over after
		// the last newline is the start of a line still in flight.
		index := bytes.IndexByte(s.pending, '\n')
		if index < 0 {
			break
		}

		line := bytes.TrimSpace(s.pending[:index])
		s.pending = s.pending[index+1:]

		s.emit(line)
	}

	return len(chunk), nil
}

// streamLine is the sliver of one stream-json record the caller can act on.
type streamLine struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

// emit reports one record. A line that will not parse is skipped rather than
// failing the turn: the answer rides on the result envelope, and the events
// are narration.
func (s *streamSink) emit(line []byte) {
	if len(line) == 0 {
		return
	}

	var record streamLine
	if json.Unmarshal(line, &record) != nil {
		return
	}

	switch record.Type {
	case "assistant":
		s.on(Event{Kind: EventAssistant, Blocks: len(record.Message.Content)})

		for _, block := range record.Message.Content {
			if block.Type == "tool_use" {
				s.on(Event{Kind: EventToolUse, Name: block.Name, Input: block.Input})
			}
		}
	case "result":
		s.result = bytes.Clone(line)
		s.on(Event{Kind: EventResult})
	}
}
