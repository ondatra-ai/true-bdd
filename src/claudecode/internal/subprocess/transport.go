// Package subprocess provides the subprocess transport implementation for Claude Code CLI.
package subprocess

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"bdd-cli/src/claudecode/internal/cli"
	"bdd-cli/src/claudecode/internal/parser"
	"bdd-cli/src/claudecode/internal/shared"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	// maxStderrReadSize limits stderr reads to prevent memory issues.
	maxStderrReadSize = 10 * 1024 // 10KB
)

const (
	// channelBufferSize is the buffer size for message and error channels.
	channelBufferSize = 10
	// terminationTimeoutSeconds is the timeout for graceful process termination.
	terminationTimeoutSeconds = 5
	// windowsOS is the GOOS value for Windows platform.
	windowsOS = "windows"
	// BUFFER FIX: Increase default buffer size to handle large responses.
	maxScanTokenSize = 10 * 1024 * 1024 // 10MB buffer (vs default 64KB)
)

// Transport implements the Transport interface using subprocess communication.
type Transport struct {
	// Process management
	cmd        *exec.Cmd
	cliPath    string
	options    *shared.Options
	closeStdin bool
	entrypoint string // CLAUDE_CODE_ENTRYPOINT value (sdk-go or sdk-go-client)

	// Connection state
	connected bool
	mu        sync.RWMutex

	// I/O streams
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *os.File

	// Message parsing
	parser *parser.Parser

	// Channels for communication
	msgChan chan shared.Message
	errChan chan error

	// Control and cleanup
	done   chan struct{}
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new subprocess transport.
func New(cliPath string, options *shared.Options, closeStdin bool, entrypoint string) *Transport {
	return &Transport{
		cliPath:    cliPath,
		options:    options,
		closeStdin: closeStdin,
		entrypoint: entrypoint,
		parser:     parser.New(),
	}
}

// IsConnected returns whether the transport is currently connected.
func (t *Transport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.connected && t.cmd != nil && t.cmd.Process != nil
}

// Connect starts the Claude CLI subprocess.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("transport already connected: %w", pkgerrors.ErrTransportAlreadyConnected)
	}

	// Set up command and working directory
	t.setupCommand(ctx)

	// Set up I/O pipes
	err := t.setupIOPipes()
	if err != nil {
		return err
	}

	// Start the process
	err = t.cmd.Start()
	if err != nil {
		t.cleanup()

		return shared.NewConnectionError(
			fmt.Sprintf("failed to start Claude CLI: %v", err),
			err,
		)
	}

	// Set up context for goroutine management
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	t.done = make(chan struct{})

	// Initialize channels
	t.msgChan = make(chan shared.Message, channelBufferSize)
	t.errChan = make(chan error, channelBufferSize)

	// Close done channel when context is cancelled
	go func() {
		<-ctx.Done()
		close(t.done)
	}()

	// Start I/O handling goroutines
	t.wg.Add(1)

	go t.handleStdout()

	// Note: Do NOT close stdin here for one-shot mode
	// The CLI still needs stdin to receive the message, even with --print flag
	// stdin will be closed after sending the message in SendMessage()

	t.connected = true

	return nil
}

// SendMessage sends a message to the CLI subprocess.
func (t *Transport) SendMessage(ctx context.Context, message shared.StreamMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.stdin == nil {
		return fmt.Errorf("transport not connected: %w", pkgerrors.ErrTransportNotConnected)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during send: %w", ctx.Err())
	default:
	}

	// Serialize message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", pkgerrors.ErrMarshalMessageFailed(err))
	}

	// Send with newline
	_, err = t.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("write message failed: %w", pkgerrors.ErrWriteMessageFailed(err))
	}

	// For one-shot mode, close stdin after sending the message
	if t.closeStdin {
		_ = t.stdin.Close()
		t.stdin = nil
	}

	return nil
}

// ReceiveMessages returns channels for receiving messages and errors.
func (t *Transport) ReceiveMessages(_ context.Context) (<-chan shared.Message, <-chan error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		// Return closed channels if not connected
		msgChan := make(chan shared.Message)
		errChan := make(chan error)

		close(msgChan)
		close(errChan)

		return msgChan, errChan
	}

	return t.msgChan, t.errChan
}

// Interrupt sends an interrupt signal to the subprocess.
func (t *Transport) Interrupt(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.cmd == nil || t.cmd.Process == nil {
		return fmt.Errorf("process not running: %w", pkgerrors.ErrProcessNotRunning)
	}

	// Windows doesn't support os.Interrupt signal
	if runtime.GOOS == windowsOS {
		return fmt.Errorf("interrupt not supported on Windows: %w", pkgerrors.ErrInterruptNotSupported)
	}

	// Send interrupt signal (Unix/Linux/macOS)
	err := t.cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("send interrupt signal: %w", err)
	}

	return nil
}

// Close terminates the subprocess connection.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil // Already closed
	}

	t.connected = false

	// Cancel context to stop goroutines
	if t.cancel != nil {
		t.cancel()
	}

	// Close stdin if open
	if t.stdin != nil {
		_ = t.stdin.Close()
		t.stdin = nil
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})

	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished gracefully
	case <-time.After(terminationTimeoutSeconds * time.Second):
		// Timeout: proceed with cleanup anyway
		// Goroutines should terminate when process is killed
	}

	// Terminate process with 5-second timeout
	var err error
	if t.cmd != nil && t.cmd.Process != nil {
		err = t.terminateProcess()
	}

	// Cleanup resources
	t.cleanup()

	return err
}

// handleStdout processes stdout in a separate goroutine
// BUFFER OVERFLOW FIX: Use custom scanner with larger buffer.
func (t *Transport) handleStdout() {
	defer t.wg.Done()
	defer close(t.msgChan)
	defer close(t.errChan)

	// Build handler chain
	chain := t.buildStdoutHandlerChain()
	scanner := t.createStdoutScanner()
	linesRead := 0

	for scanner.Scan() {
		if t.isDone() {
			return
		}

		linesRead++

		ctx := &ProcessContext{Line: scanner.Text()}
		if !chain.Handle(ctx, t) {
			return
		}
	}

	t.handleScannerError(scanner.Err())

	// If no lines were read, the process likely exited with an error.
	// Read stderr and send it as a ProcessError so callers get the real error.
	if linesRead == 0 {
		stderr := t.readStderr()
		if stderr != "" {
			t.sendError(shared.NewProcessError(
				"claude process exited without output", 0, stderr,
			))
		}
	}
}

// buildStdoutHandlerChain creates the chain of responsibility for processing stdout.
func (t *Transport) buildStdoutHandlerChain() StdoutHandler {
	emptyFilter := &EmptyLineFilter{}
	lineParser := NewLineParser(t.parser)
	errorSender := &ErrorSender{}
	messageSender := &MessageSender{}

	emptyFilter.SetNext(lineParser).SetNext(errorSender).SetNext(messageSender)

	return emptyFilter
}

// createStdoutScanner creates a scanner with larger buffer to handle large responses.
func (t *Transport) createStdoutScanner() *bufio.Scanner {
	scanner := bufio.NewScanner(t.stdout)
	buf := make([]byte, 0, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	return scanner
}

// isDone checks if the transport is shutting down.
func (t *Transport) isDone() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// sendError sends an error to the error channel.
func (t *Transport) sendError(err error) bool {
	select {
	case t.errChan <- err:
		return true
	case <-t.done:
		return false
	}
}

// sendMessages sends parsed messages to the message channel.
func (t *Transport) sendMessages(messages []shared.Message) bool {
	for _, msg := range messages {
		if msg != nil {
			select {
			case t.msgChan <- msg:
			case <-t.done:
				return false
			}
		}
	}

	return true
}

// handleScannerError handles scanner errors.
func (t *Transport) handleScannerError(err error) {
	if err != nil {
		select {
		case t.errChan <- pkgerrors.ErrStdoutScannerFailed(err):
		case <-t.done:
		}
	}
}

// isProcessAlreadyFinishedError checks if an error indicates the process has already terminated.
// This follows the Python SDK pattern of suppressing "process not found" type errors.
func isProcessAlreadyFinishedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	return strings.Contains(errStr, "process already finished") ||
		strings.Contains(errStr, "process already released") ||
		strings.Contains(errStr, "no child processes") ||
		strings.Contains(errStr, "signal: killed")
}

// sendTerminationSignal sends SIGTERM to the process and handles failures.
func (t *Transport) sendTerminationSignal() error {
	err := t.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		// If process is already finished, that's success
		if isProcessAlreadyFinishedError(err) {
			return nil
		}
		// If SIGTERM fails for other reasons, try SIGKILL immediately
		killErr := t.cmd.Process.Kill()
		if killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return fmt.Errorf("kill process after SIGTERM failure: %w", killErr)
		}

		return nil // Don't return error for expected termination
	}

	return nil
}

// terminateProcess implements the 5-second SIGTERM → SIGKILL sequence.
func (t *Transport) terminateProcess() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}

	err := t.sendTerminationSignal()
	if err != nil {
		return err
	}

	return t.waitForProcessTermination()
}

// waitForProcessTermination waits for process to exit with timeout handling.
func (t *Transport) waitForProcessTermination() error {
	done := make(chan error, 1)
	cmd := t.cmd

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return t.handleProcessExit(err)
	case <-time.After(terminationTimeoutSeconds * time.Second):
		return terminateProcess(cmd, done, "timeout")
	case <-t.done:
		return terminateProcess(cmd, done, "context cancellation")
	}
}

// handleProcessExit checks if process exit was expected.
func (t *Transport) handleProcessExit(err error) error {
	if err != nil && strings.Contains(err.Error(), "signal:") {
		return nil
	}

	return err
}

// setupCommand builds and configures the command with arguments and environment.
func (t *Transport) setupCommand(ctx context.Context) {
	// Build command with all options
	args := cli.BuildCommand(t.cliPath, t.options, t.closeStdin)
	// Create command context - subprocess execution required for Claude CLI SDK
	t.cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	// Set up environment
	t.cmd.Env = append(os.Environ(), "CLAUDE_CODE_ENTRYPOINT="+t.entrypoint)

	// Set working directory if specified
	if t.options != nil && t.options.Cwd != nil {
		err := cli.ValidateWorkingDirectory(*t.options.Cwd)
		if err == nil {
			t.cmd.Dir = *t.options.Cwd
		}
	}
}

// setupIOPipes sets up stdin, stdout, and stderr pipes for the command.
func (t *Transport) setupIOPipes() error {
	var err error

	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe failed: %w", pkgerrors.ErrCreateStdinPipeFailed(err))
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", pkgerrors.ErrCreateStdoutPipeFailed(err))
	}

	// Isolate stderr using temporary file to prevent deadlocks
	// This matches Python SDK pattern to avoid subprocess pipe deadlocks
	t.stderr, err = os.CreateTemp("", "claude_stderr_*.log")
	if err != nil {
		return fmt.Errorf("create stderr file failed: %w", pkgerrors.ErrCreateStderrFileFailed(err))
	}

	t.cmd.Stderr = t.stderr

	return nil
}

// readStderr reads and returns the content of the stderr temp file.
// Must be called before cleanup() which removes the file.
func (t *Transport) readStderr() string {
	if t.stderr == nil {
		return ""
	}

	_, seekErr := t.stderr.Seek(0, io.SeekStart)
	if seekErr != nil {
		return ""
	}

	content, err := io.ReadAll(io.LimitReader(t.stderr, maxStderrReadSize))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

// cleanup cleans up all resources.
func (t *Transport) cleanup() {
	if t.stdout != nil {
		_ = t.stdout.Close()
		t.stdout = nil
	}

	if t.stderr != nil {
		// Graceful cleanup matching Python SDK pattern
		// Python: except Exception: pass
		_ = t.stderr.Close()
		_ = os.Remove(t.stderr.Name()) // Ignore cleanup errors
		t.stderr = nil
	}

	// Reset state
	t.cmd = nil
}
