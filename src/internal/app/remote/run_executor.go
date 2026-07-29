package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
)

const (
	streamChunkSize   = 4096
	eventTailInterval = 120 * time.Millisecond

	// terminalDeliveryTimeout bounds the final terminal-event flush when
	// the run's ctx is already cancelled (Ctrl+C interrupt): the child is
	// already down, so a fresh context lets the interrupted envelope reach
	// the server instead of aborting on the cancelled ctx (plan §3.2).
	terminalDeliveryTimeout = 30 * time.Second

	streamStdout = "stdout"
	streamStderr = "stderr"

	eventTypeOutput       = "output"
	eventTypePrompt       = "prompt"
	eventTypeTerminal     = "terminal"
	eventTypeLockAcquired = "lock_acquired"

	childEventPrompt = "prompt"
	childEventResult = "result"
)

// childEvent is the child-emitted JSONL shape the remote tails from the
// event-channel file (plan §3.2). The `result` event carries the FULL
// engine result — outcome, whether finalization (the post-walk write)
// succeeded, and any detail — so the remote can synthesize the terminal
// envelope without erasing the underlying facts (finding 7).
type childEvent struct {
	Type           string         `json:"type"`
	PromptID       string         `json:"prompt_id"`
	Kind           string         `json:"kind"`
	Payload        map[string]any `json:"payload"`
	Outcome        string         `json:"outcome"`
	FinalizationOK bool           `json:"finalization_ok"`
	Detail         string         `json:"detail"`
}

// RunExecutor runs one dispatched command as a child process, streams
// its stdout/stderr and event-channel events into the remote-owned
// (run_id, seq) stream, relays prompt answers to the child's stdin, and
// synthesizes the terminal envelope (plan §3.2).
type RunExecutor struct {
	binPath  string
	folder   string
	run      RunSpec
	children *ChildrenRegistry
	sender   *Outbox

	mu             sync.Mutex
	stdin          io.WriteCloser
	pgid           int
	lastAnswerSeq  int
	lastPromptKind string

	// result is the child's engine result, written by the tailer goroutine
	// and read after tailer.Wait() establishes the happens-before barrier.
	result childResult

	// interrupted is the CAUSAL shutdown flag (NICE-1): true only when
	// watchShutdown actually selected ctx.Done() and initiated escalation,
	// so a natural completion racing a late cancel is not misclassified
	// interrupted. atomic because watchShutdown runs in its own goroutine.
	interrupted atomic.Bool
}

// NewRunExecutor builds an executor for one run.
func NewRunExecutor(
	client *ServerClient,
	sessionID, binPath, folder string,
	run RunSpec,
	children *ChildrenRegistry,
) *RunExecutor {
	return &RunExecutor{
		binPath:  binPath,
		folder:   folder,
		run:      run,
		children: children,
		sender:   newOutbox(client, sessionID, run.RunID, filepath.Join(folder, "tmp"), defaultOutboxOptions()),
	}
}

// RunID returns the id of the run being executed.
func (e *RunExecutor) RunID() string {
	return e.run.RunID
}

// Execute runs the command to a terminal envelope, always sending
// exactly one terminal event last.
func (e *RunExecutor) Execute(ctx context.Context) {
	lock, err := AcquireFolderLock(e.lockPath())
	if err != nil {
		e.sendTerminal(ctx, lockFailureEnvelope(err))

		return
	}
	defer lock.Release()

	// Emit the AUTHORITATIVE flock proof immediately after acquisition and
	// BEFORE argument construction / spawn (plan §3.7 / finding 5). The
	// server abandons stale older folder runs on THIS event — so a run that
	// took the lock and then fails to spawn still resolves the old run,
	// which incidental output/prompt no longer stands in for.
	e.sendLockAcquired(ctx)

	child, eventsPath, err := e.spawnChild(lock)
	if err != nil {
		slog.Error("remote: spawn child failed", "run", e.run.RunID, "error", err)
		e.sendTerminal(ctx, terminalEnvelope{outcome: outcomeError, detail: detailSpawn})

		return
	}

	e.children.Add(childEntry{
		PGID:          child.pgid,
		StartIdentity: processStartIdentity(child.pgid),
		RunID:         e.run.RunID,
	})
	defer e.children.Remove(child.pgid)

	// Remove the per-run event JSONL after the tailer's final drain
	// (NICE-2 lifecycle hygiene): the events were already relayed into the
	// durable (run_id, seq) stream, so the file is no longer needed.
	defer func() { _ = os.Remove(eventsPath) }()

	waitErr := e.stream(ctx, child, eventsPath)
	e.sendTerminal(ctx, e.classifyTerminal(ctx, waitErr))
}

// DeliverAnswer writes a browser-accepted answer to the child's stdin
// exactly once per answer_seq. The answer_seq is memoized (and reported as
// consumed) BEFORE the bytes are written (plan §3.3), so a lost-ack
// redelivery of the same answer is acknowledgement-only — stdin is never
// written twice per answer_seq. A malformed answer is rejected by
// remote-side hygiene (plan §3.2) without being written.
func (e *RunExecutor) DeliverAnswer(answer AnswerMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stdin == nil || answer.AnswerSeq <= e.lastAnswerSeq {
		return
	}

	err := validateAnswer(e.lastPromptKind, answer.Value)
	if err != nil {
		slog.Warn("remote: rejecting invalid answer",
			"run", e.run.RunID, "seq", answer.AnswerSeq, "error", err)

		return
	}

	// Memoize BEFORE writing bytes: even if the write below fails or the
	// process dies mid-write, the same answer_seq is never re-consumed.
	e.lastAnswerSeq = answer.AnswerSeq
	e.sender.SetAnswersConsumed(answer.AnswerSeq)

	_, err = io.WriteString(e.stdin, formatAnswer(e.lastPromptKind, answer.Value))
	if err != nil {
		slog.Warn("remote: write answer to child stdin failed", "run", e.run.RunID, "error", err)
	}
}

// Interrupt forwards SIGINT to the child's process group and closes its
// stdin (plan §3.2 basic path — a blocked child sees EOF ⇒ Exit).
func (e *RunExecutor) Interrupt() {
	e.mu.Lock()
	pgid := e.pgid
	stdin := e.stdin
	e.mu.Unlock()

	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGINT)
	}

	if stdin != nil {
		_ = stdin.Close()
	}
}

// classifyTerminal picks the terminal envelope. A real Claude call
// interrupted mid-flight makes the child exit with a non-zero ERROR code
// (its evaluator's claude call failed), not a signal death, so plan §3.2's
// "killed by signal ⇒ interrupted" precedence does not always win that
// race; the remote-level cancellation is the authoritative interrupt
// signal. The full exit/result envelope is always attached for diagnostics.
func (e *RunExecutor) classifyTerminal(ctx context.Context, waitErr error) terminalEnvelope {
	env := synthesizeEnvelope(waitErr, e.result)

	if e.wasInterrupted(ctx, waitErr) {
		env.outcome = outcomeInterrupted
		env.detail = ""
	}

	return env
}

// wasInterrupted decides whether the run was torn down by a remote-level
// cancellation (Ctrl+C / SIGTERM) rather than by its own completion.
//
// It is interrupted when watchShutdown actually SELECTED ctx.Done() and
// initiated escalation (the causal flag — NICE-1), OR when the run ctx was
// cancelled and the child did NOT finish as a clean natural completion.
//
// The causal flag alone would be fragile: in a real Ctrl+C the child can
// die (non-zero, no result) from the group signal BEFORE watchShutdown
// observes ctx.Done(), which would misclassify a genuine interrupt as
// error(no_result). The second clause keeps that robust. The "clean
// natural completion" guard (a present result AND a clean exit) is what
// prevents a LATE, unrelated cancellation from overwriting a real result —
// the defect NICE-1 targets. This is a deliberately stronger design than a
// bare causal flag (documented in the fix report).
func (e *RunExecutor) wasInterrupted(ctx context.Context, waitErr error) bool {
	if e.interrupted.Load() {
		return true
	}

	if ctx.Err() == nil {
		return false
	}

	cleanNaturalCompletion := waitErr == nil && e.result.present

	return !cleanNaturalCompletion
}

// spawnChild launches the command child in its own process group with the
// folder flock fd inherited (plan §3.7). It returns the managed child and
// the per-run event-channel path the tailer reads.
func (e *RunExecutor) spawnChild(lock *FolderLock) (*managedChild, string, error) {
	args, err := commandArgs(e.run)
	if err != nil {
		return nil, "", err
	}

	eventsPath := e.eventsPath()
	_ = os.Remove(eventsPath)

	child, err := spawnProcessGroup(spawnConfig{
		binPath:  e.binPath,
		args:     args,
		env:      childEnv(eventsPath),
		dir:      e.folder,
		lockFile: lock.File(),
	})
	if err != nil {
		return nil, "", err
	}

	e.mu.Lock()
	e.stdin = child.stdin
	e.pgid = child.pgid
	e.mu.Unlock()

	return child, eventsPath, nil
}

// stream reads stdout/stderr and the event file concurrently, waits for
// the child to exit, and returns the wait error. A watcher goroutine
// escalates SIGINT → SIGTERM → SIGKILL if ctx is cancelled (plan §3.2).
// stdout/stderr are drained BEFORE Wait (the StdoutPipe contract); the
// event tailer runs until signalled, then does a final drain so the
// result event lands.
func (e *RunExecutor) stream(ctx context.Context, child *managedChild, eventsPath string) error {
	go watchShutdown(ctx, child, &e.interrupted)

	var readers sync.WaitGroup

	readers.Add(2) //nolint:mnd // stdout + stderr
	go e.pump(ctx, &readers, child.stdout, streamStdout)
	go e.pump(ctx, &readers, child.stderr, streamStderr)

	tailDone := make(chan struct{})

	var tailer sync.WaitGroup

	tailer.Add(1)
	go e.tailEvents(ctx, &tailer, eventsPath, tailDone)

	readers.Wait()

	waitErr := child.Wait()

	close(tailDone)
	tailer.Wait()

	if waitErr != nil {
		return fmt.Errorf("child wait: %w", waitErr)
	}

	return nil
}

// watchShutdown escalates the child's shutdown when ctx is cancelled, or
// returns quietly once the child exits on its own. It records the CAUSAL
// interrupt flag (NICE-1) ONLY when it actually selected ctx.Done() and
// initiated escalation, so a natural completion that merely races a later
// cancellation is never reported as interrupted.
func watchShutdown(ctx context.Context, child *managedChild, interrupted *atomic.Bool) {
	select {
	case <-ctx.Done():
		interrupted.Store(true)
		child.Escalate(defaultEscalateConfig())
	case <-child.done:
	}
}

func (e *RunExecutor) pump(ctx context.Context, group *sync.WaitGroup, reader io.Reader, stream string) {
	defer group.Done()

	buffer := make([]byte, streamChunkSize)

	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			e.sendOutput(ctx, stream, string(buffer[:count]))
		}

		if err != nil {
			return
		}
	}
}

// sendLockAcquired posts the explicit lock_acquired proof event (finding 5).
func (e *RunExecutor) sendLockAcquired(ctx context.Context) {
	err := e.sender.Send(ctx, OutEvent{Type: eventTypeLockAcquired})
	if err != nil {
		slog.Warn("remote: send lock_acquired failed", "run", e.run.RunID, "error", err)
	}
}

func (e *RunExecutor) sendOutput(ctx context.Context, stream, data string) {
	err := e.sender.Send(ctx, OutEvent{Type: eventTypeOutput, Stream: stream, Data: data})
	if err != nil {
		slog.Warn("remote: send output failed", "run", e.run.RunID, "error", err)
	}
}

func (e *RunExecutor) tailEvents(ctx context.Context, group *sync.WaitGroup, path string, done <-chan struct{}) {
	defer group.Done()

	var offset int64

	for {
		offset = e.drainEvents(ctx, path, offset)

		select {
		case <-done:
			e.drainEvents(ctx, path, offset)

			return
		case <-time.After(eventTailInterval):
		}
	}
}

// drainEvents reads and handles the complete lines appended past offset,
// returning the new offset (advanced only past whole lines).
func (e *RunExecutor) drainEvents(ctx context.Context, path string, offset int64) int64 {
	file, err := os.Open(path) //nolint:gosec // path is a remote-controlled tmp file
	if err != nil {
		return offset
	}
	defer func() { _ = file.Close() }()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return offset
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return offset
	}

	lines := bytes.Split(data, []byte("\n"))
	consumed := 0

	for index := range len(lines) - 1 {
		e.handleChildLine(ctx, lines[index])
		consumed += len(lines[index]) + 1
	}

	return offset + int64(consumed)
}

func (e *RunExecutor) handleChildLine(ctx context.Context, line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}

	var event childEvent

	err := json.Unmarshal(line, &event)
	if err != nil {
		slog.Warn("remote: unparseable child event", "run", e.run.RunID, "line", string(line))

		return
	}

	switch event.Type {
	case childEventPrompt:
		e.relayPrompt(ctx, event)
	case childEventResult:
		// Retain the WHOLE result (finding 7). Written in the tailer
		// goroutine; read after tailer.Wait()'s happens-before barrier.
		e.result = childResult{
			present:        true,
			outcome:        event.Outcome,
			finalizationOK: event.FinalizationOK,
			detail:         event.Detail,
		}
	}
}

func (e *RunExecutor) relayPrompt(ctx context.Context, event childEvent) {
	e.mu.Lock()
	e.lastPromptKind = event.Kind
	e.mu.Unlock()

	err := e.sender.Send(ctx, OutEvent{
		Type:     eventTypePrompt,
		PromptID: event.PromptID,
		Kind:     event.Kind,
		Payload:  event.Payload,
	})
	if err != nil {
		slog.Warn("remote: relay prompt failed", "run", e.run.RunID, "error", err)
	}
}

// sendTerminal posts the terminal event (last, highest seq) then drives a
// final bounded flush so a healthy server acks it and the spool is
// deleted; a persistent outage leaves the spool for startup replay.
//
// On a Ctrl+C interrupt the run's ctx is already cancelled — that
// cancellation is precisely what tore the child down — but the terminal
// (interrupted) envelope MUST still reach the server, or the run hangs
// non-terminal until a later incarnation replays the spool. So when the
// run ctx is already done, delivery switches to a fresh bounded context
// (plan §3.2: the interrupt path completes with the envelope delivered).
func (e *RunExecutor) sendTerminal(ctx context.Context, envelope terminalEnvelope) {
	if ctx.Err() != nil {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(context.Background(), terminalDeliveryTimeout)
		defer cancel()
	}

	err := e.sender.Send(ctx, OutEvent{
		Type:           eventTypeTerminal,
		Outcome:        envelope.outcome,
		ErrorDetail:    envelope.detail,
		EngineOutcome:  envelope.engineOutcome,
		FinalizationOK: envelope.finalizationOK,
		ExitCode:       envelope.exitCode,
		Signal:         envelope.signal,
	})
	if err != nil {
		slog.Error("remote: send terminal failed", "run", e.run.RunID, "error", err)
	}

	err = e.sender.Flush(ctx)
	if err != nil {
		slog.Warn("remote: final flush incomplete; spool retained for replay",
			"run", e.run.RunID, "error", err)
	}
}

func (e *RunExecutor) lockPath() string {
	return filepath.Join(e.folder, "tmp", "true-bdd-harness.lock")
}

func (e *RunExecutor) eventsPath() string {
	return filepath.Join(e.folder, "tmp", e.run.RunID+"-events.jsonl")
}

// commandPipes wires the child's stdin/stdout/stderr pipes.
func commandPipes(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	return stdin, stdout, stderr, nil
}

// childEnv strips CLAUDECODE (production stripping — plan §3.2/§4.4) and
// points the child at its per-run event-channel file.
func childEnv(eventsPath string) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+1)

	for _, entry := range base {
		if strings.HasPrefix(entry, "CLAUDECODE=") {
			continue
		}

		if strings.HasPrefix(entry, events.EventsFileEnv+"=") {
			continue
		}

		out = append(out, entry)
	}

	return append(out, events.EventsFileEnv+"="+eventsPath)
}

// formatAnswer frames an answer for the child's line-based collector.
// Freetext gets a terminating blank line (the collector reads until an
// empty line — plan §3.2 / A2).
func formatAnswer(kind, value string) string {
	base := value
	if !strings.HasSuffix(base, "\n") {
		base += "\n"
	}

	if kind == string(events.KindFreetext) {
		base += "\n"
	}

	return base
}

// lockFailureEnvelope classifies a lock-acquisition failure.
func lockFailureEnvelope(err error) terminalEnvelope {
	if errors.Is(err, errFolderLocked) {
		return terminalEnvelope{outcome: outcomeError, detail: detailFolderLocked}
	}

	return terminalEnvelope{outcome: outcomeError, detail: detailSpawn}
}
