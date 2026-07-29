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
	resp, err := a.client.Poll(ctx, PollRequest{SessionID: a.sessionID, ActiveRunID: a.activeRunID()})
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
func (a *Agent) uploadInventory(ctx context.Context) {
	req := a.buildOrReuseInventory()

	_, err := a.client.PostInventory(ctx, *req)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("remote: inventory upload failed; retained for retry", "error", err)
		}

		return // keep pendingInv set so the next attempt retries this snapshot
	}

	a.mu.Lock()
	a.pendingInv = nil
	a.mu.Unlock()
}

// buildOrReuseInventory returns the retained unacked upload if one exists,
// otherwise captures a fresh ticket, scans, and retains the new upload.
func (a *Agent) buildOrReuseInventory() *InventoryRequest {
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

	snapshot := inventory.Scan(a.folder)

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
