// Package parser provides JSON message parsing functionality with speculative parsing and buffer management.
package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"bdd-cli/src/claudecode/internal/shared"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

// ErrSkippedMessage is returned when a message has an unknown type and should be skipped.
var ErrSkippedMessage = errors.New("skipped unknown message type")

// Sentinel errors for internal skip conditions.
var (
	errIncompleteJSON      = errors.New("incomplete JSON")
	errSkippedContentBlock = errors.New("skipped unknown content block type")
)

const (
	// MaxBufferSize is the maximum buffer size to prevent memory exhaustion (1MB).
	MaxBufferSize = 1024 * 1024
)

// Parser handles JSON message parsing with speculative parsing and buffer management.
// It implements the same speculative parsing strategy as the Python SDK.
type Parser struct {
	buffer           strings.Builder
	maxBufferSize    int
	mu               sync.Mutex // Thread safety
	requiredStrategy FieldParsingStrategy
	optionalStrategy FieldParsingStrategy
}

// New creates a new JSON parser with default buffer size and strategies.
func New() *Parser {
	return &Parser{
		maxBufferSize:    MaxBufferSize,
		requiredStrategy: &RequiredFieldsStrategy{},
		optionalStrategy: &OptionalFieldsStrategy{},
	}
}

// ProcessLine processes a line of JSON input with speculative parsing.
// Handles multiple JSON objects on single line and embedded newlines.
func (p *Parser) ProcessLine(line string) ([]shared.Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	var messages []shared.Message

	// Handle multiple JSON objects on single line by splitting on newlines
	jsonLines := strings.Split(line, "\n")
	for _, jsonLine := range jsonLines {
		jsonLine = strings.TrimSpace(jsonLine)
		if jsonLine == "" {
			continue
		}

		// Process each JSON line with speculative parsing (unlocked version)
		msg, err := p.processJSONLineUnlocked(jsonLine)
		if err != nil {
			// Incomplete JSON and unknown types are not real errors — skip silently
			if errors.Is(err, errIncompleteJSON) || errors.Is(err, ErrSkippedMessage) {
				continue
			}

			return messages, err
		}

		if msg != nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// ParseMessage parses a raw JSON object into the appropriate Message type.
// Implements type discrimination based on the "type" field.
func (p *Parser) ParseMessage(data map[string]any) (shared.Message, error) {
	msgType, ok := data["type"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("missing or invalid type field", data)
	}

	switch msgType {
	case shared.MessageTypeUser:
		return p.parseUserMessage(data)
	case shared.MessageTypeAssistant:
		return p.parseAssistantMessage(data)
	case shared.MessageTypeSystem:
		return p.parseSystemMessage(data)
	case shared.MessageTypeResult:
		return p.parseResultMessage(data)
	default:
		slog.Debug("skipping unknown message type", "type", msgType)

		return nil, ErrSkippedMessage
	}
}

// Reset clears the internal buffer.
func (p *Parser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer.Reset()
}

// BufferSize returns the current buffer size.
func (p *Parser) BufferSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.buffer.Len()
}

// processJSONLineUnlocked attempts to parse accumulated buffer as JSON using speculative parsing.
// This is the core of the speculative parsing strategy from the Python SDK.
// Must be called with mutex already held.
func (p *Parser) processJSONLineUnlocked(jsonLine string) (shared.Message, error) {
	p.buffer.WriteString(jsonLine)

	// Check buffer size limit
	if p.buffer.Len() > p.maxBufferSize {
		bufferSize := p.buffer.Len()
		p.buffer.Reset()

		return nil, shared.NewJSONDecodeError(
			"buffer overflow",
			0,
			pkgerrors.ErrBufferSizeExceeded(bufferSize, p.maxBufferSize),
		)
	}

	// Attempt speculative JSON parsing
	var rawData map[string]any

	bufferContent := p.buffer.String()

	err := json.Unmarshal([]byte(bufferContent), &rawData)
	if err != nil {
		// JSON is incomplete - continue accumulating
		// This is NOT an error condition in speculative parsing!
		return nil, errIncompleteJSON
	}

	// Successfully parsed complete JSON - reset buffer and parse message
	p.buffer.Reset()

	return p.ParseMessage(rawData)
}

// parseUserMessage parses a user message from raw JSON data.
func (p *Parser) parseUserMessage(data map[string]any) (*shared.UserMessage, error) {
	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("user message missing message field", data)
	}

	content := messageData["content"]
	if content == nil {
		return nil, shared.NewMessageParseError("user message missing content field", data)
	}

	// Handle both string content and array of content blocks
	switch contentValue := content.(type) {
	case string:
		// String content - create directly
		return &shared.UserMessage{
			Content: contentValue,
		}, nil
	case []any:
		// Array of content blocks
		var blocks []shared.ContentBlock

		for index, blockData := range contentValue {
			block, err := p.parseContentBlock(blockData)
			if err != nil {
				if errors.Is(err, errSkippedContentBlock) {
					continue
				}

				return nil, fmt.Errorf("parse content block failed: %w", pkgerrors.ErrParseContentBlockFailed(index, err))
			}

			blocks = append(blocks, block)
		}

		return &shared.UserMessage{
			Content: blocks,
		}, nil
	default:
		return nil, shared.NewMessageParseError("invalid user message content type", data)
	}
}

// parseAssistantMessage parses an assistant message from raw JSON data.
func (p *Parser) parseAssistantMessage(data map[string]any) (*shared.AssistantMessage, error) {
	messageData, found := data["message"].(map[string]any)
	if !found {
		return nil, shared.NewMessageParseError("assistant message missing message field", data)
	}

	contentArray, found := messageData["content"].([]any)
	if !found {
		return nil, shared.NewMessageParseError("assistant message content must be array", data)
	}

	model, found := messageData["model"].(string)
	if !found {
		return nil, shared.NewMessageParseError("assistant message missing model field", data)
	}

	var blocks []shared.ContentBlock

	for index, blockData := range contentArray {
		block, err := p.parseContentBlock(blockData)
		if err != nil {
			if errors.Is(err, errSkippedContentBlock) {
				continue
			}

			return nil, fmt.Errorf("parse content block failed: %w", pkgerrors.ErrParseContentBlockFailed(index, err))
		}

		blocks = append(blocks, block)
	}

	return &shared.AssistantMessage{
		Content: blocks,
		Model:   model,
	}, nil
}

// parseSystemMessage parses a system message from raw JSON data.
func (p *Parser) parseSystemMessage(data map[string]any) (*shared.SystemMessage, error) {
	subtype, ok := data["subtype"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("system message missing subtype field", data)
	}

	return &shared.SystemMessage{
		Subtype: subtype,
		Data:    data, // Preserve all original data
	}, nil
}

// parseResultMessage parses a result message from raw JSON data.
func (p *Parser) parseResultMessage(data map[string]any) (*shared.ResultMessage, error) {
	result := &shared.ResultMessage{}

	// Parse required fields using injected strategy
	err := p.requiredStrategy.ParseFields(data, result)
	if err != nil {
		return nil, fmt.Errorf("parse required fields: %w", err)
	}

	// Parse optional fields using injected strategy
	err = p.optionalStrategy.ParseFields(data, result)
	if err != nil {
		return nil, fmt.Errorf("parse optional fields: %w", err)
	}

	return result, nil
}

// parseContentBlock parses a content block based on its type field.
func (p *Parser) parseContentBlock(blockData any) (shared.ContentBlock, error) {
	data, valid := blockData.(map[string]any)
	if !valid {
		return nil, shared.NewMessageParseError("content block must be an object", blockData)
	}

	blockType, ok := data["type"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("content block missing type field", data)
	}

	switch blockType {
	case shared.ContentBlockTypeText:
		return p.parseTextBlock(data)
	case shared.ContentBlockTypeThinking:
		return p.parseThinkingBlock(data)
	case shared.ContentBlockTypeToolUse:
		return p.parseToolUseBlock(data)
	case shared.ContentBlockTypeToolResult:
		return p.parseToolResultBlock(data)
	default:
		slog.Debug("skipping unknown content block type", "type", blockType)

		return nil, errSkippedContentBlock
	}
}

func (p *Parser) parseTextBlock(data map[string]any) (shared.ContentBlock, error) {
	text, ok := data["text"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("text block missing text field", data)
	}

	return &shared.TextBlock{Text: text}, nil
}

func (p *Parser) parseThinkingBlock(data map[string]any) (shared.ContentBlock, error) {
	thinking, ok := data["thinking"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("thinking block missing thinking field", data)
	}

	signature, _ := data["signature"].(string) // Optional field

	return &shared.ThinkingBlock{
		Thinking:  thinking,
		Signature: signature,
	}, nil
}

func (p *Parser) parseToolUseBlock(data map[string]any) (shared.ContentBlock, error) {
	identifier, found := data["id"].(string)
	if !found {
		return nil, shared.NewMessageParseError("tool_use block missing id field", data)
	}

	name, found := data["name"].(string)
	if !found {
		return nil, shared.NewMessageParseError("tool_use block missing name field", data)
	}

	input, _ := data["input"].(map[string]any) // Optional field
	if input == nil {
		input = make(map[string]any)
	}

	return &shared.ToolUseBlock{
		ToolUseID: identifier,
		Name:      name,
		Input:     input,
	}, nil
}

func (p *Parser) parseToolResultBlock(data map[string]any) (shared.ContentBlock, error) {
	toolUseID, ok := data["tool_use_id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("tool_result block missing tool_use_id field", data)
	}

	var isError *bool

	if isErrorValue, exists := data["is_error"]; exists {
		if b, ok := isErrorValue.(bool); ok {
			isError = &b
		}
	}

	return &shared.ToolResultBlock{
		ToolUseID: toolUseID,
		Content:   data["content"],
		IsError:   isError,
	}, nil
}
