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

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/store"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/events"
)

const (
	streamChunkSize   = 4096
	eventTailInterval = 120 * time.Millisecond

	// Control-event append is NEVER-DROP (finding 7): a transient persistence
	// failure is retried a bounded number of times before failing closed.
	controlAppendRetries    = 5
	controlAppendRetryDelay = 25 * time.Millisecond

	streamStdout = "stdout"
	streamStderr = "stderr"

	// Event type names — the CLI store's generic per-run ledger (plan §1.2).
	eventTypeOutput       = "output"
	eventTypePrompt       = "prompt"
	eventTypeTerminal     = "terminal"
	eventTypeLockAcquired = "lock_acquired"

	childEventPrompt = "prompt"
	childEventResult = "result"
)

// childEvent is the child-emitted JSONL shape the remote tails from the
// event-channel file (plan §1.2). The `result` event carries the FULL engine
// result — outcome, whether finalization succeeded, and any detail — so the
// remote synthesizes the terminal envelope without erasing the underlying
// facts (finding 7).
type childEvent struct {
	Type           string         `json:"type"`
	PromptID       string         `json:"prompt_id"`
	Kind           string         `json:"kind"`
	Payload        map[string]any `json:"payload"`
	Outcome        string         `json:"outcome"`
	FinalizationOK bool           `json:"finalization_ok"`
	Detail         string         `json:"detail"`
}

// RunExecutor runs one dispatched command as a child process, streams its
// stdout/stderr and event-channel events into the CLI store's per-run event
// ledger (plan §1.2), relays prompt answers to the child's stdin, and
// synthesizes the terminal envelope (plan §3.2). It gates the spawn behind the
// store's command lease (plan §1.5) and records the child's group identity on
// the run row BEFORE streaming so a crash never leaves an untracked group
// (plan §1.6).
type RunExecutor struct {
	store    *store.DB
	locks    *store.Locks
	children *ChildrenRegistry
	binPath  string
	folder   string
	ownerID  string
	run      RunSpec

	mu    sync.Mutex
	stdin io.WriteCloser
	pgid  int

	// result is the child's engine result, written by the tailer goroutine and
	// read after tailer.Wait() establishes the happens-before barrier.
	result childResult

	// interrupted is the CAUSAL shutdown flag: true only when watchShutdown
	// actually selected ctx.Done() and initiated escalation.
	interrupted atomic.Bool

	// controlFailed latches when a CONTROL event could not be persisted (after
	// retries): the run fails closed — the child is interrupted and the tailer
	// stops advancing, so it never blocks on an invisible prompt (finding 7).
	controlFailed atomic.Bool
}

// NewRunExecutor builds an executor for one run.
func NewRunExecutor(
	database *store.DB,
	locks *store.Locks,
	children *ChildrenRegistry,
	binPath, folder, ownerID string,
	run RunSpec,
) *RunExecutor {
	return &RunExecutor{
		store:    database,
		locks:    locks,
		children: children,
		binPath:  binPath,
		folder:   folder,
		ownerID:  ownerID,
		run:      run,
	}
}

// RunID returns the id of the run being executed.
func (e *RunExecutor) RunID() string {
	return e.run.RunID
}

// Execute takes the command lease, spawns the child, streams its output into
// the store, and appends exactly one terminal event last. It BLOCKS until the
// child exits, holding the lease so the folder stays reserved for this command
// (plan §1.5); releasing it frees the folder for the next command (P10).
func (e *RunExecutor) Execute(ctx context.Context) {
	lease, outcome := e.locks.BeginCommand()

	switch outcome {
	case lockOutcomeOK:
		defer lease.Release()
	case lockOutcomeFolderLocked:
		// A sibling command already holds the folder — terminal folder_locked
		// without a spawn (plan §1.5, P15).
		e.appendTerminal(lockFailureEnvelope(errFolderLocked))

		return
	case lockOutcomeTimeout:
		// A bounded scan-drain wait exceeded: a 504-class dispatch failure,
		// classified as error(timeout) — NOT a spawn error and NOT a false
		// folder_locked (plan §1.5). (The 201 dispatch reply was already sent
		// before the spawn, so this surfaces on the run row, not the HTTP status.)
		e.appendTerminal(terminalEnvelope{outcome: outcomeError, detail: detailTimeout})

		return
	default:
		// Any other unexpected non-ok outcome is a spawn-class error.
		e.appendTerminal(terminalEnvelope{outcome: outcomeError, detail: detailSpawn})

		return
	}

	// The command lease holds the folder flock; publish the lock-proof
	// transition (queued → running) into the store (plan §1.2). If it cannot be
	// persisted, FAIL CLOSED without spawning — a child must never run without
	// its lock-proof transition recorded (finding 7).
	if !e.appendLockAcquired() {
		slog.Error("remote: lock_acquired unpersistable — failing closed without spawn", "run", e.run.RunID)
		e.appendTerminal(terminalEnvelope{outcome: outcomeError, detail: detailSpawn})

		return
	}

	child, eventsPath, err := e.spawnChild()
	if err != nil {
		slog.Error("remote: spawn child failed", "run", e.run.RunID, "error", err)
		e.appendTerminal(terminalEnvelope{outcome: outcomeError, detail: detailSpawn})

		return
	}

	// The gated supervisor's identity was recorded BEFORE it was released to run
	// the real command (spawnChild), so the mutating group is already tracked.
	defer e.children.Remove(child.pgid)

	// The events were relayed into the durable store ledger, so drop the
	// per-run JSONL after the final drain.
	defer func() { _ = os.Remove(eventsPath) }()

	waitErr := e.stream(ctx, child, eventsPath)
	e.appendTerminal(e.classifyTerminal(ctx, waitErr))
}

// DeliverAnswer writes a store-accepted answer to the child's stdin (plan
// §1.4). At-most-once delivery is enforced by the store's delivery_started_at
// commit, so the executor simply validates and writes. A malformed answer is
// rejected by remote-side hygiene without being written.
func (e *RunExecutor) DeliverAnswer(kind, value string) error {
	e.mu.Lock()
	stdin := e.stdin
	e.mu.Unlock()

	if stdin == nil {
		return errNoChildStdin
	}

	validationErr := validateAnswer(kind, value)
	if validationErr != nil {
		return fmt.Errorf("deliver answer: %w", validationErr)
	}

	_, err := io.WriteString(stdin, formatAnswer(kind, value))
	if err != nil {
		return fmt.Errorf("write answer to child stdin: %w", err)
	}

	return nil
}

// Interrupt forwards SIGINT to the child's process group and closes its stdin
// (plan §3.2 basic path — a blocked child sees EOF ⇒ Exit).
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

// classifyTerminal picks the terminal envelope. A remote-level cancellation
// (Ctrl+C / SIGTERM) is the authoritative interrupt signal; version and
// prompt-probe emit no engine result, so their clean exit is `ok` (plan §3.1).
func (e *RunExecutor) classifyTerminal(ctx context.Context, waitErr error) terminalEnvelope {
	env := synthesizeEnvelope(waitErr, e.result)

	if e.wasInterrupted(ctx, waitErr) {
		env.outcome = outcomeInterrupted
		env.detail = ""

		return env
	}

	// version / prompt-probe emit no engine result; a clean exit ⇒ ok.
	if !e.result.present && !resultExpected(e.run.Command) {
		code, signaled, _ := exitInfo(waitErr)
		if !signaled && code == 0 {
			env.outcome = outcomeOK
			env.detail = ""
		}
	}

	return env
}

// wasInterrupted decides whether the run was torn down by a remote-level
// cancellation rather than by its own completion (see the design note retained
// from v1: a late unrelated cancel must not overwrite a real result).
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

// spawnChild launches the command via the RESIDENT GATED supervisor (plan §1.6,
// finding 4): the supervisor becomes the process-group leader and blocks on its
// release gate; this method records + fsyncs the supervisor's verifiable group
// identity on the run row AND the per-owner pidfile BEFORE releasing it to exec
// the real command, so a crash never leaves an untracked mutating group. A
// bookkeeping failure FAILS CLOSED — the gate is aborted and the supervisor
// exits without ever running the command. The folder flock is held by the
// command lease, so the child does not inherit a lock fd in v2.
func (e *RunExecutor) spawnChild() (*managedChild, string, error) {
	args, err := commandArgs(e.run)
	if err != nil {
		return nil, "", err
	}

	eventsPath := e.eventsPath()
	_ = os.Remove(eventsPath)

	child, err := spawnGatedGroup(spawnConfig{
		binPath: e.binPath,
		args:    args,
		env:     childEnv(eventsPath),
		dir:     e.folder,
	})
	if err != nil {
		return nil, "", err
	}

	// Record the RESIDENT supervisor's identity BEFORE releasing it (finding 4).
	identity := processStartIdentity(child.pgid)

	recErr := e.recordGroupIdentity(child.pgid, identity)
	if recErr != nil {
		child.abortGate()
		_ = child.Wait()

		return nil, "", recErr
	}

	// Bookkeeping durable → release the supervisor to exec the real command.
	relErr := child.Release()
	if relErr != nil {
		child.abortGate()
		e.children.Remove(child.pgid)
		_ = child.Wait()

		return nil, "", relErr
	}

	e.mu.Lock()
	e.stdin = child.stdin
	e.pgid = child.pgid
	e.mu.Unlock()

	return child, eventsPath, nil
}

// recordGroupIdentity durably records the supervisor's {pgid, start_identity}
// on the run row (WAL-durable) and the per-owner pidfile (fsynced), so boot
// reconciliation can verify or clean up the group (finding 4). A run-row write
// failure fails closed.
func (e *RunExecutor) recordGroupIdentity(pgid int, identity string) error {
	err := e.store.Exec(
		`UPDATE runs SET child_pgid = ?, child_start_identity = ? WHERE id = ?`,
		pgid, identity, e.run.RunID,
	)
	if err != nil {
		return fmt.Errorf("record group identity: %w", err)
	}

	e.children.Add(childEntry{PGID: pgid, StartIdentity: identity, RunID: e.run.RunID})

	return nil
}

// stream reads stdout/stderr and the event file concurrently, waits for the
// child to exit, and returns the wait error. A watcher goroutine escalates
// SIGINT → SIGTERM → SIGKILL if ctx is cancelled (plan §3.2).
func (e *RunExecutor) stream(ctx context.Context, child *managedChild, eventsPath string) error {
	go watchShutdown(ctx, child, &e.interrupted)

	var readers sync.WaitGroup

	readers.Add(2) //nolint:mnd // stdout + stderr
	go e.pump(&readers, child.stdout, streamStdout)
	go e.pump(&readers, child.stderr, streamStderr)

	tailDone := make(chan struct{})

	var tailer sync.WaitGroup

	tailer.Add(1)
	go e.tailEvents(&tailer, eventsPath, tailDone)

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
// interrupt flag ONLY when it actually selected ctx.Done().
func watchShutdown(ctx context.Context, child *managedChild, interrupted *atomic.Bool) {
	select {
	case <-ctx.Done():
		interrupted.Store(true)
		child.Escalate(defaultEscalateConfig())
	case <-child.done:
	}
}

func (e *RunExecutor) pump(group *sync.WaitGroup, reader io.Reader, stream string) {
	defer group.Done()

	buffer := make([]byte, streamChunkSize)

	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			e.appendOutput(stream, string(buffer[:count]))
		}

		if err != nil {
			return
		}
	}
}

func (e *RunExecutor) tailEvents(group *sync.WaitGroup, path string, done <-chan struct{}) {
	defer group.Done()

	var offset int64

	for {
		offset = e.drainEvents(path, offset)

		select {
		case <-done:
			e.drainEvents(path, offset)

			return
		case <-time.After(eventTailInterval):
		}
	}
}

// lineResult reports how the tailer must treat a processed child line.
type lineResult int

const (
	// lineAdvance: the line was persisted, legitimately dropped (run terminal),
	// a result, or garbage — advance the tail offset past it.
	lineAdvance lineResult = iota
	// lineFailClosed: a CONTROL event could not be persisted — fail closed, and
	// do NOT advance past it (finding 7).
	lineFailClosed
)

// drainEvents reads and handles the complete lines appended past offset. The
// offset advances past a line ONLY after its control event durably committed
// (or was legitimately dropped); a control event that cannot be persisted fails
// the run CLOSED and stops the drain there — the offset never advances past a
// lost control event (finding 7).
func (e *RunExecutor) drainEvents(path string, offset int64) int64 {
	if e.controlFailed.Load() {
		return offset
	}

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
		if e.handleChildLine(lines[index]) == lineFailClosed {
			// A control event is unpersistable: fail closed and STOP advancing,
			// so nothing after the lost event is silently consumed.
			e.failClosed()

			break
		}

		consumed += len(lines[index]) + 1
	}

	return offset + int64(consumed)
}

func (e *RunExecutor) handleChildLine(line []byte) lineResult {
	if len(bytes.TrimSpace(line)) == 0 {
		return lineAdvance
	}

	var event childEvent

	err := json.Unmarshal(line, &event)
	if err != nil {
		slog.Warn("remote: unparseable child event", "run", e.run.RunID, "line", string(line))

		return lineAdvance
	}

	switch event.Type {
	case childEventPrompt:
		return e.appendPrompt(event)
	case childEventResult:
		// Retain the WHOLE result (finding 7). Written in the tailer goroutine;
		// read after tailer.Wait()'s happens-before barrier.
		e.result = childResult{
			present:        true,
			outcome:        event.Outcome,
			finalizationOK: event.FinalizationOK,
			detail:         event.Detail,
		}

		return lineAdvance
	default:
		return lineAdvance
	}
}

// appendControl persists a control event, retrying transient failures a bounded
// number of times (finding 7). It returns persisted=true on commit; terminal=
// true when the run is already terminal/gone (a legitimate, safe drop); and
// (false,false) when the event remained unpersistable after every retry.
func (e *RunExecutor) appendControl(event store.EventInput) (bool, bool) {
	for range controlAppendRetries {
		out := e.store.AppendEvent(e.run.RunID, event)
		if !out.Rejected {
			return true, false
		}

		if out.Terminal {
			// The run is already terminal/gone — dropping the event is safe.
			return false, true
		}

		time.Sleep(controlAppendRetryDelay)
	}

	return false, false
}

// failClosed latches the control-failure flag and interrupts the child so it
// never blocks on an invisible prompt (finding 7).
func (e *RunExecutor) failClosed() {
	if e.controlFailed.Swap(true) {
		return
	}

	slog.Error("remote: control event unpersistable — failing the run closed", "run", e.run.RunID)
	e.Interrupt()
}

// appendLockAcquired publishes the lock-proof transition into the store,
// retrying transient failures. Returns whether it was persisted (finding 7).
func (e *RunExecutor) appendLockAcquired() bool {
	persisted, _ := e.appendControl(store.EventInput{Type: eventTypeLockAcquired, Control: true})

	return persisted
}

// appendOutput appends one stdout/stderr chunk and enforces the per-run byte
// budget (plan §1.3).
func (e *RunExecutor) appendOutput(stream, data string) {
	e.store.AppendEvent(e.run.RunID, store.EventInput{Type: eventTypeOutput, Stream: stream, Data: data})
	e.store.EnforceByteBudget(e.run.RunID)
}

// appendPrompt publishes a child prompt into the store ledger; the store's
// projection records the pending prompt row (plan §1.2/§1.4). A prompt is a
// CONTROL event: if it cannot be persisted (after retries) the tail must NOT
// advance past it — otherwise the child blocks forever on a prompt the browser
// can never see (finding 7).
func (e *RunExecutor) appendPrompt(event childEvent) lineResult {
	payloadJSON := ""

	if event.Payload != nil {
		raw, err := json.Marshal(event.Payload)
		if err == nil {
			payloadJSON = string(raw)
		}
	}

	persisted, terminal := e.appendControl(store.EventInput{
		Type:     eventTypePrompt,
		PromptID: event.PromptID,
		Kind:     event.Kind,
		Payload:  payloadJSON,
		Control:  true,
	})
	if persisted || terminal {
		return lineAdvance
	}

	slog.Warn("remote: prompt append unpersistable — failing closed", "run", e.run.RunID, "prompt", event.PromptID)

	return lineFailClosed
}

// appendTerminal writes the terminal event last (the badge classification) and
// enriches the retained diagnostic columns the generic terminal projection
// does not set (plan §1.2 / finding 7), then enforces run retention.
func (e *RunExecutor) appendTerminal(env terminalEnvelope) {
	out := e.store.AppendEvent(e.run.RunID, store.EventInput{
		Type:        eventTypeTerminal,
		Outcome:     env.outcome,
		ErrorDetail: env.detail,
		Control:     true,
	})
	if out.Rejected {
		// The run is already terminal (e.g. boot-reconciled to abandoned); do
		// not overwrite it.
		return
	}

	_ = e.store.Exec(
		`UPDATE runs SET engine_outcome = ?, finalization_ok = ?, exit_code = ?, signal = ? WHERE id = ?`,
		nullString(env.engineOutcome), nullBool(env.finalizationOK),
		nullIntPtr(env.exitCode), nullString(env.signal), e.run.RunID,
	)

	e.store.EnforceRetention()
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

// childEnv strips CLAUDECODE (production stripping — plan §3.2/§4.4) and points
// the child at its per-run event-channel file.
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

// formatAnswer frames an answer for the child's line-based collector. Freetext
// gets a terminating blank line (the collector reads until an empty line).
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

// resultExpected reports whether the command is an engine command that emits a
// `result` event; version and prompt-probe do not (plan §3.1/§4).
func resultExpected(command string) bool {
	switch command {
	case commandVersion, commandPromptProbe:
		return false
	default:
		return true
	}
}

// nullString maps "" to a SQL NULL, else the value.
func nullString(s string) any {
	if s == "" {
		return nil
	}

	return s
}

// nullBool maps a nil *bool to SQL NULL, else the bool.
func nullBool(b *bool) any {
	if b == nil {
		return nil
	}

	return *b
}

// nullIntPtr maps a nil *int to SQL NULL, else the int.
func nullIntPtr(i *int) any {
	if i == nil {
		return nil
	}

	return *i
}
