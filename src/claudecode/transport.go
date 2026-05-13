package claudecode

import (
	"context"

	"bdd-cli/src/claudecode/internal/shared"
)

// Transport defines the interface for Claude Code CLI communication.
type Transport interface {
	Connect(ctx context.Context) error
	SendMessage(ctx context.Context, message StreamMessage) error
	ReceiveMessages(ctx context.Context) (<-chan Message, <-chan error)
	Interrupt(ctx context.Context) error
	Close() error
}

// StreamMessage is an alias for the shared StreamMessage type.
type StreamMessage = shared.StreamMessage

// MessageIterator is an alias for the shared MessageIterator type.
type MessageIterator = shared.MessageIterator
