package claudecode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"bdd-cli/src/claudecode/internal/cli"
	"bdd-cli/src/claudecode/internal/subprocess"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const defaultSessionID = "default"

// ErrNoMoreMessages indicates the message iterator has no more messages.
var ErrNoMoreMessages = errors.New("no more messages")

// Client provides bidirectional streaming communication with Claude Code CLI.
type Client interface {
	Connect(ctx context.Context, prompt ...StreamMessage) error
	Disconnect() error
	Query(ctx context.Context, prompt string) error
	QueryWithSession(ctx context.Context, prompt string, sessionID string) error
	QueryStream(ctx context.Context, messages <-chan StreamMessage) error
	ReceiveMessages(ctx context.Context) <-chan Message
	ReceiveResponse(ctx context.Context) MessageIterator
	Interrupt(ctx context.Context) error
}

// ClientImpl implements the Client interface.
type ClientImpl struct {
	mu        sync.RWMutex
	transport Transport
	options   *Options
	connected bool
	msgChan   <-chan Message
	errChan   <-chan error
}

// NewClient creates a new Client with the given options.
func NewClient(opts ...Option) Client {
	options := NewOptions(opts...)
	client := &ClientImpl{
		options: options,
	}

	return client
}

// WithClient provides Go-idiomatic resource management equivalent to Python SDK's async context manager.
// It automatically connects to Claude Code CLI, executes the provided function, and ensures proper cleanup.
// This eliminates the need for manual Connect/Disconnect calls and prevents resource leaks.
//
// The function follows Go's established resource management patterns using defer for guaranteed cleanup,
// similar to how database connections, files, and other resources are typically managed in Go.
//
// Example - Basic usage:
//
//	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
//	    return client.Query(ctx, "What is 2+2?")
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Example - With configuration options:
//
//	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
//	    if err := client.Query(ctx, "Calculate the area of a circle with radius 5"); err != nil {
//	        return err
//	    }
//
//	    // Process responses
//	    for msg := range client.ReceiveMessages(ctx) {
//	        if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
//	            fmt.Println("Claude:", assistantMsg.Content[0].(*claudecode.TextBlock).Text)
//	        }
//	    }
//	    return nil
//	}, claudecode.WithSystemPrompt("You are a helpful math tutor"),
//	   claudecode.WithAllowedTools("Read", "Write"))
//
// The client will be automatically connected before callback is called and disconnected after callback returns,
// even if callback returns an error or panics. This provides 100% functional parity with Python SDK's
// 'async with ClaudeSDKClient()' pattern while using idiomatic Go resource management.
//
// Parameters:
//   - ctx: Context for connection management and cancellation
//   - callback: Function to execute with the connected client
//   - opts: Optional client configuration options
//
// Returns an error if connection fails or if callback returns an error.
// Disconnect errors are handled gracefully without overriding the original error from callback.
func WithClient(ctx context.Context, callback func(Client) error, opts ...Option) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context error before client connect: %w", ctx.Err())
	}

	client := NewClient(opts...)

	err := client.Connect(ctx)
	if err != nil {
		return pkgerrors.ErrConnectClientFailed(err)
	}

	defer func() {
		// Following Go idiom: cleanup errors don't override the original error
		// This matches patterns in database/sql, os.File, and other stdlib packages
		disconnectErr := client.Disconnect()
		if disconnectErr != nil {
			// Log cleanup errors but don't return them to preserve the original error
			// This follows the standard Go pattern for resource cleanup
			_ = disconnectErr // Explicitly acknowledge we're ignoring this error
		}
	}()

	return callback(client)
}

// Connect establishes a connection to the Claude Code CLI.
func (c *ClientImpl) Connect(ctx context.Context, _ ...StreamMessage) error {
	// Check context before acquiring lock
	if ctx.Err() != nil {
		return fmt.Errorf("context error before connect: %w", ctx.Err())
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check context again after acquiring lock
	if ctx.Err() != nil {
		return fmt.Errorf("context error after lock: %w", ctx.Err())
	}

	// Validate configuration before connecting
	err := c.validateOptions()
	if err != nil {
		return pkgerrors.ErrInvalidConfigurationError(err)
	}

	// Create default subprocess transport directly (like Python SDK)
	cliPath, err := cli.FindCLI()
	if err != nil {
		return pkgerrors.ErrCLINotFoundError(err)
	}

	// Create subprocess transport for streaming mode (closeStdin=false)
	c.transport = subprocess.New(cliPath, c.options, false, "sdk-go-client")

	// Connect the transport
	err = c.transport.Connect(ctx)
	if err != nil {
		return pkgerrors.ErrConnectTransportFailed(err)
	}

	// Get message channels
	c.msgChan, c.errChan = c.transport.ReceiveMessages(ctx)

	c.connected = true

	return nil
}

// Disconnect closes the connection to the Claude Code CLI.
func (c *ClientImpl) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transport != nil && c.connected {
		err := c.transport.Close()
		if err != nil {
			return pkgerrors.ErrCloseTransportFailed(err)
		}
	}

	c.connected = false
	c.transport = nil
	c.msgChan = nil
	c.errChan = nil

	return nil
}

// Query sends a simple text query using the default session.
// This is equivalent to QueryWithSession(ctx, prompt, "default").
//
// Example:
//
//	client.Query(ctx, "What is Go?")
func (c *ClientImpl) Query(ctx context.Context, prompt string) error {
	return c.queryWithSession(ctx, prompt, defaultSessionID)
}

// QueryWithSession sends a simple text query using the specified session ID.
// Each session maintains its own conversation context, allowing for isolated
// conversations within the same client connection.
//
// If sessionID is empty, it defaults to "default".
//
// Example:
//
//	client.QueryWithSession(ctx, "Remember this", "my-session")
//	client.QueryWithSession(ctx, "What did I just say?", "my-session") // Remembers context
//	client.Query(ctx, "What did I just say?")                          // Won't remember, different session
func (c *ClientImpl) QueryWithSession(ctx context.Context, prompt string, sessionID string) error {
	// Use default session if empty session ID provided
	if sessionID == "" {
		sessionID = defaultSessionID
	}

	return c.queryWithSession(ctx, prompt, sessionID)
}

// QueryStream sends a stream of messages.
func (c *ClientImpl) QueryStream(ctx context.Context, messages <-chan StreamMessage) error {
	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return pkgerrors.ErrClientNotConnected
	}

	// Send messages from channel in a goroutine
	go func() {
		for {
			select {
			case msg, ok := <-messages:
				if !ok {
					return // Channel closed
				}

				err := transport.SendMessage(ctx, msg)
				if err != nil {
					// Log error but continue processing
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// ReceiveMessages returns a channel of incoming messages.
func (c *ClientImpl) ReceiveMessages(_ context.Context) <-chan Message {
	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	msgChan := c.msgChan
	c.mu.RUnlock()

	if !connected || msgChan == nil {
		// Return closed channel if not connected
		closedChan := make(chan Message)
		close(closedChan)

		return closedChan
	}

	// Return the transport's message channel directly
	return msgChan
}

// ReceiveResponse returns an iterator for the response messages.
func (c *ClientImpl) ReceiveResponse(_ context.Context) MessageIterator {
	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	msgChan := c.msgChan
	errChan := c.errChan
	c.mu.RUnlock()

	if !connected || msgChan == nil {
		return nil
	}

	// Create a simple iterator over the message channel
	return &clientIterator{
		msgChan: msgChan,
		errChan: errChan,
	}
}

// Interrupt sends an interrupt signal to stop the current operation.
func (c *ClientImpl) Interrupt(ctx context.Context) error {
	// Check context before proceeding
	if ctx.Err() != nil {
		return fmt.Errorf("context error before interrupt: %w", ctx.Err())
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return pkgerrors.ErrClientNotConnected
	}

	err := transport.Interrupt(ctx)
	if err != nil {
		return fmt.Errorf("interrupt transport: %w", err)
	}

	return nil
}

// validateOptions validates the client configuration options.
func (c *ClientImpl) validateOptions() error {
	if c.options == nil {
		return nil // Nil options are acceptable (use defaults)
	}

	// Validate working directory
	if c.options.Cwd != nil {
		_, err := os.Stat(*c.options.Cwd)
		if os.IsNotExist(err) {
			return pkgerrors.ErrWorkingDirectoryDoesNotExist(*c.options.Cwd)
		}
	}

	// Validate max turns
	if c.options.MaxTurns < 0 {
		return pkgerrors.ErrMaxTurnsMustBeNonNegative(c.options.MaxTurns)
	}

	// Validate permission mode
	if c.options.PermissionMode != nil {
		validModes := map[PermissionMode]bool{
			PermissionModeDefault:           true,
			PermissionModeAcceptEdits:       true,
			PermissionModePlan:              true,
			PermissionModeBypassPermissions: true,
		}
		if !validModes[*c.options.PermissionMode] {
			return pkgerrors.ErrInvalidPermissionModeValue(string(*c.options.PermissionMode))
		}
	}

	return nil
}

// queryWithSession sends a query message with an explicit session ID.
func (c *ClientImpl) queryWithSession(ctx context.Context, prompt string, sessionID string) error {
	// Check context before proceeding
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled before query: %w", ctx.Err())
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return pkgerrors.ErrClientNotConnected
	}

	// Check context again after acquiring connection info
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled after connection check: %w", ctx.Err())
	}

	// Create user message in Python SDK compatible format
	streamMsg := StreamMessage{
		Type: "user",
		Message: map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
		ParentToolUseID: nil,
		SessionID:       sessionID,
	}

	// Send message via transport (without holding mutex to avoid blocking other operations)
	err := transport.SendMessage(ctx, streamMsg)
	if err != nil {
		return fmt.Errorf("send message via transport: %w", err)
	}

	return nil
}

// clientIterator implements MessageIterator for client message reception.
type clientIterator struct {
	msgChan <-chan Message
	errChan <-chan error
	closed  bool
}

func (ci *clientIterator) Next(ctx context.Context) (Message, error) {
	if ci.closed {
		return nil, ErrNoMoreMessages
	}

	select {
	case msg, ok := <-ci.msgChan:
		if !ok {
			ci.closed = true

			return nil, ErrNoMoreMessages
		}

		return msg, nil
	case err := <-ci.errChan:
		ci.closed = true

		return nil, err
	case <-ctx.Done():
		ci.closed = true

		return nil, fmt.Errorf("context cancelled during message iteration: %w", ctx.Err())
	}
}

func (ci *clientIterator) Close() error {
	ci.closed = true

	return nil
}
