package subprocess

import (
	"bdd-cli/src/claudecode/internal/parser"
	"bdd-cli/src/claudecode/internal/shared"
)

// ProcessContext holds data passed through the handler chain.
type ProcessContext struct {
	Line     string
	Messages []shared.Message
	Error    error
}

// StdoutHandler defines the interface for stdout processing handlers.
type StdoutHandler interface {
	SetNext(handler StdoutHandler) StdoutHandler
	Handle(ctx *ProcessContext, t *Transport) bool
}

// BaseStdoutHandler provides default chaining behavior.
type BaseStdoutHandler struct {
	next StdoutHandler
}

// SetNext sets the next handler in the chain.
func (h *BaseStdoutHandler) SetNext(handler StdoutHandler) StdoutHandler {
	h.next = handler

	return handler
}

// callNext invokes the next handler in the chain.
func (h *BaseStdoutHandler) callNext(ctx *ProcessContext, t *Transport) bool {
	if h.next != nil {
		return h.next.Handle(ctx, t)
	}

	return true
}

// EmptyLineFilter filters out empty lines from processing.
type EmptyLineFilter struct {
	BaseStdoutHandler
}

// Handle processes empty line filtering logic.
func (h *EmptyLineFilter) Handle(ctx *ProcessContext, t *Transport) bool {
	if ctx.Line == "" {
		return true
	}

	return h.callNext(ctx, t)
}

// LineParser parses stdout lines into messages.
type LineParser struct {
	BaseStdoutHandler

	parser *parser.Parser
}

// NewLineParser creates a new line parser handler.
func NewLineParser(p *parser.Parser) *LineParser {
	return &LineParser{parser: p}
}

// Handle parses the line and stores messages in context.
func (h *LineParser) Handle(ctx *ProcessContext, transport *Transport) bool {
	messages, err := h.parser.ProcessLine(ctx.Line)
	if err != nil {
		ctx.Error = err

		return h.callNext(ctx, transport)
	}

	ctx.Messages = messages

	return h.callNext(ctx, transport)
}

// ErrorSender sends errors to the error channel.
type ErrorSender struct {
	BaseStdoutHandler
}

// Handle sends error if present in context.
func (h *ErrorSender) Handle(ctx *ProcessContext, t *Transport) bool {
	if ctx.Error != nil {
		return t.sendError(ctx.Error)
	}

	return h.callNext(ctx, t)
}

// MessageSender sends parsed messages to the message channel.
type MessageSender struct {
	BaseStdoutHandler
}

// Handle sends all messages in context to the channel.
func (h *MessageSender) Handle(ctx *ProcessContext, t *Transport) bool {
	if ctx.Error != nil {
		return true
	}

	return t.sendMessages(ctx.Messages)
}
