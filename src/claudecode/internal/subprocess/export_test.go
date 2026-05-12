package subprocess

import (
	"os"

	"bdd-cli/src/claudecode/internal/shared"
)

// NewTestTransportWithChannels creates a Transport with initialized channels for testing.
// This helper allows black-box tests to verify channel behavior without accessing unexported fields.
func NewTestTransportWithChannels() *Transport {
	return &Transport{
		errChan: make(chan error, 1),
		msgChan: make(chan shared.Message, 10),
		done:    make(chan struct{}),
	}
}

// CloseTestTransport safely closes all channels in a test Transport.
func CloseTestTransport(t *Transport) {
	close(t.done)
	close(t.errChan)
	close(t.msgChan)
}

// GetTestTransportErrChan returns the error channel from a Transport for testing.
func GetTestTransportErrChan(t *Transport) <-chan error {
	return t.errChan
}

// GetTestTransportMsgChan returns the message channel from a Transport for testing.
func GetTestTransportMsgChan(t *Transport) <-chan shared.Message {
	return t.msgChan
}

// SetTestTransportStderr sets a stderr file on a test Transport for readStderr testing.
func SetTestTransportStderr(t *Transport, f *os.File) {
	t.stderr = f
}

// ReadStderrForTest exposes readStderr for testing.
func ReadStderrForTest(t *Transport) string {
	return t.readStderr()
}
