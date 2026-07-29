// Package remote implements the `true-bdd remote` host-folder agent
// (plan §3.1–§3.3). It runs inside a host project folder, connects OUT
// to the harness server, registers a session, uploads an inventory
// snapshot, then polls for dispatched commands — executing each as an
// isolated child process whose stdout/stderr and structured events are
// relayed to the server, with the interactive `--fix` prompt loop
// bridged to the browser. The agent constructs NO bootstrap container,
// so it runs honestly in a bare folder (plan §3.1).
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
)

const (
	pollInterval     = time.Second
	runChanBuffer    = 1
	tmpDirPerm       = 0o700
	registerAttempts = 5
	registerBackoff  = 500 * time.Millisecond
	// maxInventoryRescans bounds the 413-driven re-scan loop so a
	// mid-flight limit change can never wedge into an infinite retry.
	maxInventoryRescans = 24
	// budgetShrinkDivisor halves the fit limit on each 413 re-scan.
	budgetShrinkDivisor = 2
	// inventoryTokenSample is a representative client token (worst-case
	// length) used only to size the request envelope when validating the
	// server cap at register — before a real token is minted.
	inventoryTokenSample = "-inv-999999999"
)

// Options configures the remote agent.
type Options struct {
	// ServerURL is the harness server the remote connects OUT to.
	ServerURL string
	// Version is the remote's reported build version.
	Version string
}

// Agent is the long-lived remote session loop: one poller goroutine
// keeps the heartbeat alive and relays run dispatches / answers /
// inventory requests, while the main loop executes at most one run at a
// time and re-inventories after each command.
type Agent struct {
	client    *ServerClient
	binPath   string
	folder    string // canonical (realpath) folder
	rawFolder string
	version   string
	children  *ChildrenRegistry

	// incarnation is the ONE stable per-process id (NICE-5): register
	// retries reuse it, so a dropped register response re-registers into
	// the SAME server session rather than orphaning one.
	incarnation string
	sessionID   string

	mu         sync.Mutex
	scanEpoch  int
	activeExec *RunExecutor
	handled    *handledRuns
	invSeq     int
	// inventoryLimit is the server's negotiated FULL-REQUEST byte limit
	// (0 = unbounded). The uploader fits the serialized full InventoryRequest
	// — snapshot PLUS the folder/session/epoch/token envelope — under it, and
	// on a 413 re-scans against a strictly smaller limit; a successfully
	// reduced limit is persisted for later refreshes (plan §1a / finding 1).
	inventoryLimit int
	// inventoryUnavailable is the terminal cannot-fit reason (e.g.
	// "limit_too_small") when the server cap is below the minimum viable
	// request; it is reported OUT OF BAND on every poll (finding 1).
	inventoryUnavailable string
	// pendingInv is a scanned-but-unacked inventory upload retained until
	// the server acks it (finding 2): a failed upload — or a manual refresh
	// whose commit was lost — is retried, never silently dropped.
	pendingInv *InventoryRequest
}

// Run resolves the host folder, builds the agent, and drives its
// register → poll → execute loop until ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	raw, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}

	canonical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		canonical = raw
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%w: %w", errMissingExecutable, err)
	}

	tmpDir := filepath.Join(canonical, "tmp")

	err = os.MkdirAll(tmpDir, tmpDirPerm)
	if err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}

	incarnation, err := randomID()
	if err != nil {
		return err
	}

	agent := &Agent{
		client:      NewServerClient(opts.ServerURL),
		binPath:     binPath,
		folder:      canonical,
		rawFolder:   raw,
		version:     opts.Version,
		children:    NewChildrenRegistry(filepath.Join(tmpDir, "true-bdd-remote-children.pids")),
		handled:     newHandledRuns(handledRunsCap),
		incarnation: incarnation,
	}

	return agent.loop(ctx)
}

func (a *Agent) loop(ctx context.Context) error {
	err := a.register(ctx)
	if err != nil {
		return err
	}

	// Replay any spooled events an earlier incarnation left unacked — a
	// crashed or killed remote's terminal event still reaches the server on
	// the next start, then the spool is removed (plan §3.6).
	replayErr := replayUnackedSpools(ctx, a.client, a.sessionID, filepath.Join(a.folder, "tmp"))
	if replayErr != nil && ctx.Err() == nil {
		slog.Warn("remote: spool replay incomplete", "error", replayErr)
	}

	a.uploadInventory(ctx)

	runCh := make(chan RunSpec, runChanBuffer)
	invCh := make(chan struct{}, 1)

	go a.pollLoop(ctx, runCh, invCh)

	for {
		select {
		case <-ctx.Done():
			a.shutdown()

			return nil
		case run := <-runCh:
			a.executeRun(ctx, run)
		case <-invCh:
			a.uploadInventory(ctx)
		}
	}
}

// register announces this incarnation, retrying with the SAME incarnation
// id so a dropped register response re-registers into the SAME server
// session rather than orphaning one (NICE-5). The server dedups by
// incarnation_id, so a retry after a lost reply is idempotent.
func (a *Agent) register(ctx context.Context) error {
	var lastErr error

	for range registerAttempts {
		resp, err := a.client.Register(ctx, RegisterRequest{
			IncarnationID:   a.incarnation,
			Folder:          a.rawFolder,
			CanonicalFolder: a.folder,
			PID:             os.Getpid(),
			Version:         a.version,
		})
		if err == nil {
			a.sessionID = resp.SessionID
			a.setScanEpoch(resp.ScanEpoch)
			a.applyInventoryLimit(resp.InventoryLimitBytes)
			slog.Info("remote registered", "session", resp.SessionID, "folder", a.folder)

			return nil
		}

		lastErr = err

		if ctx.Err() != nil {
			return fmt.Errorf("register cancelled: %w", ctx.Err())
		}

		waitErr := sleepCtx(ctx, registerBackoff)
		if waitErr != nil {
			return waitErr
		}
	}

	return fmt.Errorf("register after %d attempts: %w", registerAttempts, lastErr)
}

func (a *Agent) pollLoop(ctx context.Context, runCh chan RunSpec, invCh chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pollOnce(ctx, runCh, invCh)
		}
	}
}

func (a *Agent) pollOnce(ctx context.Context, runCh chan RunSpec, invCh chan struct{}) {
	resp, err := a.client.Poll(ctx, PollRequest{
		SessionID:            a.sessionID,
		ActiveRunID:          a.activeRunID(),
		InventoryUnavailable: a.getInventoryUnavailable(),
	})
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("remote: poll failed", "error", err)
		}

		return
	}

	if resp.ScanEpoch > 0 {
		a.setScanEpoch(resp.ScanEpoch)
	}

	if resp.Answer != nil {
		if executor := a.getActive(); executor != nil {
			executor.DeliverAnswer(*resp.Answer)
		}
	}

	if resp.Run != nil {
		a.dispatchRun(*resp.Run, runCh)
	}

	if resp.WantInventory && a.getActive() == nil {
		requestInventory(invCh)
	}
}

// dispatchRun hands a freshly-delivered run to the main loop exactly
// once. handled dedups redelivery until the run is claimed; activeExec
// blocks a second concurrent run.
func (a *Agent) dispatchRun(run RunSpec, runCh chan RunSpec) {
	a.mu.Lock()

	if a.activeExec != nil || a.handled.has(run.RunID) {
		a.mu.Unlock()

		return
	}

	a.handled.add(run.RunID)
	a.mu.Unlock()

	select {
	case runCh <- run:
	default:
		a.mu.Lock()
		a.handled.remove(run.RunID)
		a.mu.Unlock()
	}
}

func (a *Agent) executeRun(ctx context.Context, run RunSpec) {
	executor := NewRunExecutor(a.client, a.sessionID, a.binPath, a.folder, run, a.children)

	a.setActive(executor)
	executor.Execute(ctx)
	a.setActive(nil)

	// Do NOT re-inventory immediately here (finding 2): the terminal event
	// flags want_inventory folder-wide on the server, so the NEXT idle poll
	// issues a FRESH epoch and drives the re-scan through that path. An
	// immediate same-epoch upload here would reuse a stale ticket and could
	// mislabel a slow scan.
}

// uploadInventory scans the folder and uploads a snapshot under a
// server-issued ticket. The ticket (epoch) is captured BEFORE scanning
// (finding 2), so a concurrent poll issuing a newer epoch during a slow
// scan cannot mislabel this older snapshot. The scanned snapshot+token is
// retained until the server acks it, so a failed upload — or a manual
// refresh whose commit was lost — is retried rather than silently dropped.
//
// The uploader fits the SERIALIZED FULL InventoryRequest (snapshot PLUS the
// folder/session/epoch/token envelope) under the server's negotiated limit
// (finding 1), not a guessed constant. A 413 (the request still exceeded the
// server's byte limit — e.g. the limit changed mid-flight) drives a
// strictly-smaller RE-SCAN rather than a verbatim retry that would wedge
// forever (plan §1a); a limit that cannot shrink further is the terminal
// cannot-fit state, reported out of band via the poll. A successfully
// reduced limit is PERSISTED so later refreshes start from it.
//
// On a successful upload the uploader also compares the acked request's
// epoch to the current scan epoch (finding 3): if a newer refresh ticket
// arrived while this older request was in flight (or waiting to retry), it
// clears the stale request and immediately re-scans for the latest epoch, so
// a manual refresh is never consumed server-side without its newer ticket
// ever being scanned.
func (a *Agent) uploadInventory(ctx context.Context) {
	if a.getInventoryUnavailable() != "" {
		return // terminal cannot-fit; the poll reports it out of band
	}

	a.mu.Lock()
	limit := a.inventoryLimit
	a.mu.Unlock()

	for attempt := 0; attempt <= maxInventoryRescans; attempt++ {
		req := a.buildOrReuseInventory(limit)

		_, err := a.client.PostInventory(ctx, *req)
		if err == nil {
			a.clearPendingInventory()
			a.setInventoryLimit(limit) // persist any reduced limit for later refreshes

			// A newer refresh ticket arrived while this request was in flight:
			// re-scan for the latest epoch instead of returning stale.
			if a.currentScanEpoch() > req.ScanEpoch {
				continue
			}

			return
		}

		if !errors.Is(err, errRequestTooLarge) {
			if ctx.Err() == nil {
				slog.Warn("remote: inventory upload failed; retained for retry", "error", err)
			}

			return // keep pendingInv set so the next attempt retries this snapshot
		}

		// 413 → re-scan against a strictly smaller full-request limit.
		next := shrinkInventoryLimit(limit, serializedRequestBytes(req))

		a.clearPendingInventory() // force a fresh, smaller scan next iteration

		if next <= 0 {
			a.setInventoryUnavailable(inventory.UnavailableLimitTooSmall)

			if ctx.Err() == nil {
				slog.Warn("remote: inventory exceeds the server limit and cannot shrink further", "limit", limit)
			}

			return
		}

		limit = next
	}
}

// clearPendingInventory drops the retained upload so the next scan is fresh.
func (a *Agent) clearPendingInventory() {
	a.mu.Lock()
	a.pendingInv = nil
	a.mu.Unlock()
}

// buildOrReuseInventory returns the retained unacked upload if one exists,
// otherwise captures a fresh ticket, scans the snapshot fitted so the FULL
// serialized request stays within the given server limit, and retains the
// new upload.
func (a *Agent) buildOrReuseInventory(limit int) *InventoryRequest {
	a.mu.Lock()
	if a.pendingInv != nil {
		req := a.pendingInv
		a.mu.Unlock()

		return req
	}

	// Capture the ticket BEFORE scanning.
	epoch := a.scanEpoch
	a.invSeq++
	token := fmt.Sprintf("%s-inv-%d", a.sessionID, a.invSeq)
	a.mu.Unlock()

	budget := snapshotBudgetFor(limit, a.sessionID, a.folder, token, epoch)
	snapshot := inventory.ScanWithBudget(a.folder, budget)

	req := &InventoryRequest{
		SessionID:       a.sessionID,
		CanonicalFolder: a.folder,
		ScanEpoch:       epoch,
		Snapshot:        snapshot,
		ClientToken:     token,
	}

	a.mu.Lock()
	a.pendingInv = req
	a.mu.Unlock()

	return req
}

// snapshotBudgetFor derives the SNAPSHOT-fit budget from the server's
// full-request limit by subtracting the exact serialized envelope
// (session id + folder + epoch + token wrapper) for this request. Zero (or a
// non-negotiated limit) means unbounded; a limit at or below the envelope
// collapses to the smallest positive budget so the scan degrades to the
// cannot-fit floor rather than scanning unbounded.
func snapshotBudgetFor(limit int, sessionID, folder, token string, epoch int) int {
	if limit <= 0 {
		return 0
	}

	budget := limit - inventoryEnvelopeBytes(sessionID, folder, token, epoch)
	if budget < 1 {
		return 1
	}

	return budget
}

// inventoryEnvelopeBytes is the exact serialized size of an InventoryRequest
// MINUS its snapshot — the folder/session/epoch/token wrapper. Because JSON
// marshals the snapshot verbatim in field order, the wrapper size is stable
// regardless of snapshot content, so subtracting an empty snapshot's bytes
// from a full request built with the SAME wrapper yields the exact envelope.
func inventoryEnvelopeBytes(sessionID, folder, token string, epoch int) int {
	full := serializedRequestBytes(&InventoryRequest{
		SessionID:       sessionID,
		CanonicalFolder: folder,
		ScanEpoch:       epoch,
		Snapshot:        inventory.Snapshot{},
		ClientToken:     token,
	})

	empty, err := json.Marshal(inventory.Snapshot{})
	if err != nil {
		return full
	}

	return full - len(empty)
}

// minViableRequestBytes is the smallest server limit that can carry even the
// always-fit floor request for this folder: the bounded floor snapshot's
// worst case plus this folder's request envelope. A server cap below it is a
// terminal cannot-fit condition validated at register (plan §1a / finding 1).
func minViableRequestBytes(sessionID, folder string) int {
	return inventory.MinInventoryFloor + inventoryEnvelopeBytes(sessionID, folder, sessionID+inventoryTokenSample, 0)
}

// shrinkInventoryLimit returns a full-request limit strictly smaller than the
// request just sent, halving toward zero. It returns 0 when the limit cannot
// shrink further (the cannot-fit terminal state).
func shrinkInventoryLimit(current, sent int) int {
	if sent <= 1 {
		return 0
	}

	target := sent / budgetShrinkDivisor
	if current > 0 && target >= current {
		target = current / budgetShrinkDivisor
	}

	if target < 1 {
		return 0
	}

	return target
}

// serializedRequestBytes is the JSON byte length of a full inventory request
// — the size the 413 re-scan shrinks below and the fit step verifies.
func serializedRequestBytes(req *InventoryRequest) int {
	data, err := json.Marshal(req)
	if err != nil {
		return 0
	}

	return len(data)
}

// applyInventoryLimit stores the server's negotiated full-request limit and
// validates it: a cap below the minimum viable request is a terminal
// cannot-fit condition reported out of band on every poll (finding 1).
func (a *Agent) applyInventoryLimit(serverLimit int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inventoryLimit = serverLimit
	if serverLimit > 0 && serverLimit < minViableRequestBytes(a.sessionID, a.folder) {
		a.inventoryUnavailable = inventory.UnavailableLimitTooSmall
	}
}

// setInventoryLimit persists the (possibly reduced) full-request limit.
func (a *Agent) setInventoryLimit(limit int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inventoryLimit = limit
}

// setInventoryUnavailable latches the terminal cannot-fit reason.
func (a *Agent) setInventoryUnavailable(reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inventoryUnavailable = reason
}

// getInventoryUnavailable reads the terminal cannot-fit reason (empty when
// the inventory can still be uploaded).
func (a *Agent) getInventoryUnavailable() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.inventoryUnavailable
}

// currentScanEpoch reads the latest scan epoch the poller has observed.
func (a *Agent) currentScanEpoch() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.scanEpoch
}

func (a *Agent) shutdown() {
	executor := a.getActive()
	if executor != nil {
		executor.Interrupt()
	}
}

func (a *Agent) setScanEpoch(epoch int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if epoch > a.scanEpoch {
		a.scanEpoch = epoch
	}
}

func (a *Agent) setActive(executor *RunExecutor) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.activeExec = executor
}

func (a *Agent) getActive() *RunExecutor {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.activeExec
}

func (a *Agent) activeRunID() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.activeExec == nil {
		return ""
	}

	return a.activeExec.RunID()
}

// requestInventory nudges the main loop to re-inventory, coalescing
// repeats (the buffered channel already holds a pending request).
func requestInventory(invCh chan struct{}) {
	select {
	case invCh <- struct{}{}:
	default:
	}
}
