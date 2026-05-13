package parser_test

import (
	"errors"
	"testing"

	"bdd-cli/src/claudecode/internal/parser"
	"bdd-cli/src/claudecode/internal/shared"
)

const (
	fieldType    = "type"
	fieldSubtype = "subtype"
)

func TestParseMessage_UnknownTypeSkipped(t *testing.T) {
	jsonParser := parser.New()

	msg, err := jsonParser.ParseMessage(map[string]any{
		fieldType: "rate_limit_event",
		"data":    map[string]any{"retry_after": 5},
	})

	if !errors.Is(err, parser.ErrSkippedMessage) {
		t.Fatalf("expected ErrSkippedMessage, got: %v", err)
	}

	if msg != nil {
		t.Fatalf("expected nil message for unknown type, got %v", msg)
	}
}

func TestParseMessage_KnownTypesStillWork(t *testing.T) {
	jsonParser := parser.New()

	msg, err := jsonParser.ParseMessage(map[string]any{
		fieldType:    "system",
		fieldSubtype: "init",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sysMsg, isSysMsg := msg.(*shared.SystemMessage)
	if !isSysMsg {
		t.Fatalf("expected *shared.SystemMessage, got %T", msg)
	}

	if sysMsg.Subtype != "init" {
		t.Fatalf("expected subtype init, got %s", sysMsg.Subtype)
	}
}

func TestParseMessage_MissingTypeFieldReturnsError(t *testing.T) {
	jsonParser := parser.New()

	_, err := jsonParser.ParseMessage(map[string]any{
		"data": "no type here",
	})
	if err == nil {
		t.Fatal("expected error for missing type field")
	}
}

func TestProcessLine_IncompleteJSONNoError(t *testing.T) {
	jsonParser := parser.New()

	// Incomplete JSON should not return an error (speculative parsing)
	msgs, err := jsonParser.ProcessLine(`{"type": "system"`)
	if err != nil {
		t.Fatalf("incomplete JSON should not return error, got: %v", err)
	}

	if len(msgs) != 0 {
		t.Fatalf("expected no messages for incomplete JSON, got %d", len(msgs))
	}

	// Buffer should retain content for next call
	if jsonParser.BufferSize() == 0 {
		t.Fatal("expected buffer to retain incomplete JSON")
	}
}

func TestParseContentBlock_UnknownTypeFiltered(t *testing.T) {
	jsonParser := parser.New()

	// Assistant message with one known and one unknown content block
	msg, err := jsonParser.ParseMessage(map[string]any{
		fieldType: "assistant",
		"message": map[string]any{
			"model": "claude-opus-4-6",
			"content": []any{
				map[string]any{fieldType: "text", "text": "hello"},
				map[string]any{fieldType: "server_tool_use", "id": "x", "name": "y"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assistantMsg, isAssistantMsg := msg.(*shared.AssistantMessage)
	if !isAssistantMsg {
		t.Fatalf("expected *shared.AssistantMessage, got %T", msg)
	}

	// Only the known text block should remain
	if len(assistantMsg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(assistantMsg.Content))
	}

	textBlock, isTextBlock := assistantMsg.Content[0].(*shared.TextBlock)
	if !isTextBlock {
		t.Fatalf("expected *shared.TextBlock, got %T", assistantMsg.Content[0])
	}

	if textBlock.Text != "hello" {
		t.Fatalf("expected text 'hello', got %q", textBlock.Text)
	}
}

func TestProcessLine_MultipleUnknownTypesDontCorruptBuffer(t *testing.T) {
	jsonParser := parser.New()

	// Process several unknown types in sequence
	unknownTypes := []string{
		`{"type":"rate_limit_event","retry_after":5}`,
		`{"type":"ping"}`,
		`{"type":"heartbeat"}`,
	}

	for _, line := range unknownTypes {
		msgs, err := jsonParser.ProcessLine(line)
		if err != nil {
			t.Fatalf("unexpected error for unknown type %s: %v", line, err)
		}

		if len(msgs) != 0 {
			t.Fatalf("expected no messages for unknown type, got %d", len(msgs))
		}
	}

	// Buffer should be clean after each complete JSON parse
	if jsonParser.BufferSize() != 0 {
		t.Fatalf("expected empty buffer after complete JSON parses, got %d", jsonParser.BufferSize())
	}

	// Now parse a valid message — should still work
	msgs, err := jsonParser.ProcessLine(`{"type":"system","subtype":"init"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}
