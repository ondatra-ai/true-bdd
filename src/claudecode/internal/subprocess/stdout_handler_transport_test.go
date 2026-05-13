package subprocess_test

import (
	"errors"
	"testing"

	"bdd-cli/src/claudecode/internal/shared"
	"bdd-cli/src/claudecode/internal/subprocess"
)

const testSubtype = "test"

// TestErrorSenderWithRealChannels tests ErrorSender with actual Transport channels.
// This test uses export_test.go helpers to verify channel behavior without accessing unexported fields.
func TestErrorSenderWithRealChannels(t *testing.T) {
	t.Run("error is sent to transport error channel", func(t *testing.T) {
		// Create a minimal Transport with channels via helper
		transport := subprocess.NewTestTransportWithChannels()
		defer subprocess.CloseTestTransport(transport)

		sender := &subprocess.ErrorSender{}
		testErr := errors.New("test error from handler")
		ctx := &subprocess.ProcessContext{Error: testErr}

		// ErrorSender should send the error to the transport
		result := sender.Handle(ctx, transport)

		// Verify the handler returned true (sendError succeeded)
		if !result {
			t.Error("expected handler to return true when error is successfully sent")
		}

		// Verify error was actually sent to channel
		errChan := subprocess.GetTestTransportErrChan(transport)
		select {
		case receivedErr := <-errChan:
			if receivedErr.Error() != testErr.Error() {
				t.Errorf("expected error %q, got %q", testErr, receivedErr)
			}
		default:
			t.Error("expected error in channel, got nothing")
		}
	})

	t.Run("no error continues to next handler", func(t *testing.T) {
		transport := subprocess.NewTestTransportWithChannels()
		defer subprocess.CloseTestTransport(transport)

		sender := &subprocess.ErrorSender{}
		mockNext := &mockTransportHandler{shouldReturn: true}
		sender.SetNext(mockNext)

		ctx := &subprocess.ProcessContext{Error: nil}

		sender.Handle(ctx, transport)

		if !mockNext.wasCalled {
			t.Error("expected next handler to be called when no error")
		}

		// Verify no error was sent
		errChan := subprocess.GetTestTransportErrChan(transport)
		select {
		case err := <-errChan:
			t.Errorf("unexpected error in channel: %v", err)
		default:
			// Expected: no error sent
		}
	})
}

// TestMessageSenderSingleMessage tests MessageSender sends one message correctly.
func TestMessageSenderSingleMessage(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	mockMsg := &shared.SystemMessage{Subtype: testSubtype}
	sender := &subprocess.MessageSender{}
	ctx := &subprocess.ProcessContext{
		Messages: []shared.Message{mockMsg},
		Error:    nil,
	}

	result := sender.Handle(ctx, transport)

	if !result {
		t.Error("expected handler to return true after sending messages")
	}

	// Verify message was sent
	msgChan := subprocess.GetTestTransportMsgChan(transport)
	select {
	case receivedMsg := <-msgChan:
		if receivedMsg == nil {
			t.Error("expected message in channel, got nil")
		}

		sysMsg, ok := receivedMsg.(*shared.SystemMessage)
		if !ok {
			t.Error("expected SystemMessage type")
		} else if sysMsg.Subtype != testSubtype {
			t.Errorf("expected subtype 'test', got %q", sysMsg.Subtype)
		}
	default:
		t.Error("expected message in channel, got nothing")
	}
}

// TestMessageSenderWithError tests that errors skip message sending.
func TestMessageSenderWithError(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	mockMsg := &shared.SystemMessage{Subtype: testSubtype}
	sender := &subprocess.MessageSender{}
	ctx := &subprocess.ProcessContext{
		Messages: []shared.Message{mockMsg},
		Error:    errors.New("context has error"),
	}

	result := sender.Handle(ctx, transport)

	if !result {
		t.Error("expected handler to return true even with error")
	}

	// Verify no message was sent
	msgChan := subprocess.GetTestTransportMsgChan(transport)
	select {
	case msg := <-msgChan:
		t.Errorf("unexpected message in channel: %v", msg)
	default:
		// Expected: no message sent
	}
}

// TestMessageSenderMultipleMessages tests sending multiple messages.
func TestMessageSenderMultipleMessages(t *testing.T) {
	transport := subprocess.NewTestTransportWithChannels()
	defer subprocess.CloseTestTransport(transport)

	msg1 := &shared.SystemMessage{Subtype: "msg1"}
	msg2 := &shared.SystemMessage{Subtype: "msg2"}

	sender := &subprocess.MessageSender{}
	ctx := &subprocess.ProcessContext{
		Messages: []shared.Message{msg1, msg2},
		Error:    nil,
	}

	sender.Handle(ctx, transport)

	// Count messages received
	msgChan := subprocess.GetTestTransportMsgChan(transport)
	count := 0

	for range 2 {
		select {
		case <-msgChan:
			count++
		default:
			// Channel empty
		}
	}

	if count != 2 {
		t.Errorf("expected 2 messages, got %d", count)
	}
}

// mockTransportHandler is used for testing handler chaining with Transport.
type mockTransportHandler struct {
	subprocess.BaseStdoutHandler

	wasCalled    bool
	shouldReturn bool
}

func (m *mockTransportHandler) Handle(_ *subprocess.ProcessContext, _ *subprocess.Transport) bool {
	m.wasCalled = true

	return m.shouldReturn
}
