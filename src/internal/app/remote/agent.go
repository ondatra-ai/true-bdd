// Package remote implements the `true-bdd remote` host-folder agent (plan
// §1–§2). It runs inside a host project folder and OWNS all run/session state
// in a per-project SQLite store (`<folder>/tmp/true-bdd-state.db`). It connects
// OUT to the STATELESS relay via register / poll / reply: the relay carries
// query / dispatch / answer work items to the CLI and relays the CLI's reply
// verbatim to the browser. The agent constructs NO bootstrap container, so it
// runs honestly in a bare folder.
package remote

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ondatra-ai/true-bdd/src/internal/app/queryserver"
	"github.com/ondatra-ai/true-bdd/src/internal/app/store"
)

const (
	tmpDirPerm = 0o700
	// dbFileName is the per-project CLI state store (plan §1.1).
	dbFileName = "true-bdd-state.db"
)

// Options configures the remote agent.
type Options struct {
	// ServerURL is the relay the remote connects OUT to.
	ServerURL string
	// Version is the remote's reported build version.
	Version string
}

// Agent is the long-lived remote session: it registers with the relay, runs a
// single-outstanding-poll loop, and dispatches each work item to a query /
// dispatch / answer handler. Writes go through the store (the single writer);
// projection reads use a second read-only handle.
type Agent struct {
	client   *RelayClient
	store    *store.DB
	locks    *store.Locks
	read     *readHandle
	children *ChildrenRegistry
	docs     *docStore
	chat     *chatHandler

	binPath   string
	folder    string // canonical (realpath) folder
	rawFolder string
	version   string

	// sessionID is the CLIENT-OWNED, process-stable id used as BOTH the relay
	// session_id AND the store owner_id (plan §1.7): a relay restart
	// re-registers with the SAME id.
	sessionID     string
	startIdentity string
	pid           int

	mu                   sync.Mutex
	epoch                int
	token                string
	inventoryBudgetBytes int
	activeExec           *RunExecutor

	// scheduler bounds concurrent work handling (mutations serialized, ≤4
	// reads, ≤1 inventory) — no unbounded goroutine per poll (finding 6).
	scheduler *workScheduler

	// runWG tracks in-flight run executors so a normal shutdown WAITS for them
	// to finish escalating their child before the process exits (finding 4).
	runWG sync.WaitGroup
}

// shutdownWait bounds how long a normal shutdown waits for active executors to
// finish tearing down their child (plan §1.6, finding 4).
const shutdownWait = 40 * time.Second

// Run resolves the host folder, opens the per-project store + locks, runs boot
// reconciliation, then drives the register → poll → reply loop until ctx is
// cancelled (plan §1–§2).
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

	agent, err := newAgent(opts, raw, canonical, binPath, tmpDir)
	if err != nil {
		return err
	}

	defer agent.closeResources()

	agent.bootReconcile()

	return agent.loop(ctx)
}

// newAgent opens the store, locks, and read handle, and mints the stable
// session id + process start identity.
func newAgent(opts Options, raw, canonical, binPath, tmpDir string) (*Agent, error) {
	dbPath := filepath.Join(tmpDir, dbFileName)

	database, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	locks, err := store.OpenLocks(tmpDir)
	if err != nil {
		_ = database.Close()

		return nil, fmt.Errorf("open locks: %w", err)
	}

	read, err := openReadHandle(dbPath)
	if err != nil {
		_ = database.Close()

		return nil, fmt.Errorf("open read handle: %w", err)
	}

	sessionID, err := randomID()
	if err != nil {
		_ = read.Close()
		_ = database.Close()

		return nil, err
	}

	pid := os.Getpid()

	return &Agent{
		client:        NewRelayClient(opts.ServerURL),
		store:         database,
		locks:         locks,
		read:          read,
		children:      NewChildrenRegistry(childrenPidfilePath(tmpDir, sessionID)),
		docs:          newDocStore(canonical),
		chat:          newChatHandler(canonical),
		binPath:       binPath,
		folder:        canonical,
		rawFolder:     raw,
		version:       opts.Version,
		sessionID:     sessionID,
		startIdentity: startIdentity(pid),
		pid:           pid,
	}, nil
}

// childrenPidfilePath is the per-owner children pidfile (plan §1.6): keying it
// on the owner id fixes the same-folder clobbering bug of the shared v1 file.
func childrenPidfilePath(tmpDir, ownerID string) string {
	return filepath.Join(tmpDir, "true-bdd-remote-children."+ownerID+".jsonl")
}

// startIdentity returns a stable-for-life process identity (ps lstart), with a
// pid+boot-time fallback when ps is unavailable (plan §1.6).
func startIdentity(pid int) string {
	if id := processStartIdentity(pid); id != "" {
		return id
	}

	return fmt.Sprintf("pid-%d-boot-%d", pid, time.Now().UnixNano())
}

// closeResources releases the store, read handle, and locks (best-effort).
func (a *Agent) closeResources() {
	if a.read != nil {
		_ = a.read.Close()
	}

	if a.store != nil {
		_ = a.store.Close()
	}
}

// loop registers, then runs the single-outstanding-poll loop until ctx is
// cancelled (plan §2). A context cancellation is a clean shutdown (nil); any
// other error propagates.
func (a *Agent) loop(ctx context.Context) error {
	if a.scheduler == nil {
		// Bounded work scheduler (finding 6): initialized here so both Run and
		// the direct-construction test path share the same bounded dispatch.
		a.scheduler = newWorkScheduler(a.handleWork)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // jitter, not crypto

	err := a.register(ctx)
	if err != nil {
		return a.gracefulOrErr(ctx, err)
	}

	attempt := 0

	for ctx.Err() == nil {
		item, status, err := a.client.Poll(ctx, a.pollRequest())
		if err != nil {
			attempt++
			_ = sleepCtx(ctx, queryserver.FullJitterBackoff(attempt, rng))

			continue
		}

		attempt = 0

		reconnErr := a.handlePollResult(ctx, item, status, rng)
		if reconnErr != nil {
			return a.gracefulOrErr(ctx, reconnErr)
		}
	}

	a.shutdown()

	return nil
}

// gracefulOrErr maps a context-cancellation error to a clean shutdown (nil),
// surfacing any other error.
func (a *Agent) gracefulOrErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		a.shutdown()

		//nolint:nilerr // a cancelled context is the graceful-shutdown signal: drop the wrapped ctx error and exit cleanly
		return nil
	}

	return err
}

// handlePollResult reacts to one poll: a 200 work item is handled in its own
// goroutine (then the loop repolls immediately — exactly one outstanding
// poll); a 204 repolls; any other status classifies into a reconnect via
// queryserver (a 404 re-registers with the SAME session id, plan §2).
func (a *Agent) handlePollResult(ctx context.Context, item *workItem, status int, rng *rand.Rand) error {
	switch {
	case status == http.StatusOK && item != nil:
		// Submit to the bounded scheduler instead of spawning an unbounded
		// goroutine (finding 6). The submit is non-blocking, so the loop still
		// re-polls immediately — exactly one outstanding poll.
		a.scheduler.Submit(ctx, *item)

		return nil
	case status == http.StatusNoContent:
		return nil
	default:
		reconnect := queryserver.ReconnectFor(queryserver.ClassifyStatus(status))

		if reconnect.Backoff {
			_ = sleepCtx(ctx, queryserver.FullJitterBackoff(0, rng))
		}

		if reconnect.ReRegister {
			return a.register(ctx)
		}

		return nil
	}
}

// register announces this incarnation (SAME session id every time) and stores
// the negotiated epoch, capability token, and reply budget. It retries forever
// with capped full-jitter backoff until success or ctx cancel (plan §2).
func (a *Agent) register(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // jitter, not crypto

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return fmt.Errorf("register cancelled: %w", ctx.Err())
		}

		resp, err := a.client.Register(ctx, registerRequest{
			SessionID:       a.sessionID,
			Folder:          a.rawFolder,
			CanonicalFolder: a.folder,
			PID:             a.pid,
			StartIdentity:   a.startIdentity,
			Version:         a.version,
		})
		if err == nil {
			a.mu.Lock()
			a.epoch = resp.ConnectionEpoch
			a.token = resp.CapabilityToken
			a.inventoryBudgetBytes = resp.InventoryBudgetBytes
			a.mu.Unlock()

			slog.Info("remote registered",
				"session", a.sessionID, "epoch", resp.ConnectionEpoch,
				"reply_budget", resp.ReplyBudgetBytes, "inventory_budget", resp.InventoryBudgetBytes,
				"folder", a.folder)

			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("register cancelled: %w", ctx.Err())
		}

		waitErr := sleepCtx(ctx, queryserver.FullJitterBackoff(attempt, rng))
		if waitErr != nil {
			return waitErr
		}
	}
}

// shutdown interrupts the active child on ctx cancel and WAITS (bounded) for
// every active executor to finish tearing its child down, so the process never
// exits leaving a mutating group mid-escalation (plan §2/§1.6, finding 4). The
// store and read handle are closed by the deferred closeResources in Run.
func (a *Agent) shutdown() {
	if exec := a.getActive(); exec != nil {
		exec.Interrupt()
	}

	done := make(chan struct{})

	go func() {
		a.runWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(shutdownWait):
		slog.Warn("remote: shutdown timed out waiting for active executors")
	}
}

// ── small synchronized accessors ──

func (a *Agent) pollRequest() pollRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	return pollRequest{SessionID: a.sessionID, ConnectionEpoch: a.epoch, CapabilityToken: a.token}
}

func (a *Agent) getToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.token
}

// getInventoryBudget returns the negotiated inventory-fit budget (the full
// request budget the snapshot is fitted under, plan §1.5).
func (a *Agent) getInventoryBudget() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.inventoryBudgetBytes
}

func (a *Agent) setActive(exec *RunExecutor) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.activeExec = exec
}

func (a *Agent) getActive() *RunExecutor {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.activeExec
}

// sleepCtx sleeps for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("remote wait cancelled: %w", ctx.Err())
	case <-time.After(duration):
		return nil
	}
}
