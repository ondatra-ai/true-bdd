// Package steps holds the bdd-web suite's step definitions: the Go code
// that executes the Given/When/Then lines of every registry scenario
// whose `service:` is bdd-web.
//
// Same runner as the CLI suite, different verbs. tests/libraries/bddgo
// knows nothing about either — it binds a step's text to a function and
// hands that function whatever state the suite chose to keep. Here that
// state is a browser page; in tests/bdd-cli/steps it is a subprocess and
// a working tree. That is the whole of what "one framework, two
// surfaces" means in practice, and it is why a scenario can be written
// once and pointed at either.
//
// This is the first slice of the Go replacement for the parked
// Playwright suite at tests/legacy/bdd-web-playwright. It covers the
// scenarios the registry assigns to bdd-web today and no more; the rest
// of that suite is ported behind it, scenario by scenario, and the
// parked tree is deleted only when nothing is left in it.
package steps

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
)

// ErrServerNotReady is returned when the application never answered
// within the boot budget.
var ErrServerNotReady = errors.New("the application did not start")

// errNotTCP guards the one impossible branch of port allocation.
var errNotTCP = errors.New("listener is not TCP")

const (
	// bootTimeout caps how long the suite waits for the app to answer
	// its first request. The build is done before this starts, so a
	// server that has not answered inside it is a server that will not.
	bootTimeout = 60 * time.Second
	// bootPollInterval is how often the boot wait re-probes.
	bootPollInterval = 200 * time.Millisecond
	// probeTimeout caps one readiness probe, so a socket that accepts
	// and then goes quiet cannot hold the whole boot wait open.
	probeTimeout = 2 * time.Second
	// buildTimeout caps `next build`. Cold, on a clean tree, it is
	// minutes; this is the point past which something is wrong rather
	// than slow.
	buildTimeout = 10 * time.Minute
	// copyTimeout caps assembling the bundle from the build output.
	copyTimeout = 2 * time.Minute
	// dirPerm is the mode for every directory this suite creates.
)

// Harness is everything the suite builds once per `go test` invocation
// and every scenario then shares: the browser, and the application under
// test with the URL it answers on.
type Harness struct {
	// Mode is live, record or replay, mirroring the CLI suite's flag.
	// The web suite has no AI turns of its own yet, so today it only
	// travels with the run for the report to state.
	Mode string
	// RepoRoot anchors every path this suite resolves.
	RepoRoot string
	// BaseURL is where the application under test answers.
	BaseURL string
	// Browser is shared; each scenario opens its own context, so cookies
	// and storage never leak from one scenario into the next.
	Browser playwright.Browser

	driver *playwright.Playwright
	server *exec.Cmd
}

// NewHarness builds the application, starts it on a free loopback port,
// installs the Playwright driver, and launches a browser — once, cached
// under the user's home, which is why this suite skips the one-minute commit gate.
func NewHarness(ctx context.Context, mode, repoRoot string) (*Harness, func(), error) {
	bundle, err := buildBundle(ctx, repoRoot)
	if err != nil {
		return nil, nil, err
	}

	port, err := freePort(ctx)
	if err != nil {
		return nil, nil, err
	}

	harness := &Harness{
		Mode:     mode,
		RepoRoot: repoRoot,
		BaseURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
	}

	err = harness.startServer(ctx, bundle, port)
	if err != nil {
		harness.stop()

		return nil, nil, err
	}

	err = harness.openBrowser(ctx)
	if err != nil {
		harness.stop()

		return nil, nil, err
	}

	return harness, harness.stop, nil
}

// buildBundle compiles the app and assembles the standalone bundle
// exactly as the Dockerfile's runtime stage does. Built from source every
// time, not cached: a stale build would make this suite's green mean nothing.
func buildBundle(ctx context.Context, repoRoot string) (string, error) {
	appDir := filepath.Join(repoRoot, "services", "bdd-web")

	buildCtx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()

	build := exec.CommandContext(buildCtx, "npm", "run", "build")
	build.Dir = appDir
	build.Stdout = console.Err()
	build.Stderr = console.Err()

	err := build.Run()
	if err != nil {
		return "", fmt.Errorf("build the application under test: %w", err)
	}

	bundle := filepath.Join(repoRoot, "tmp", "bdd-web-bundle")

	err = disk.RemoveTree(bundle)
	if err != nil {
		return "", fmt.Errorf("clear the bundle dir: %w", err)
	}

	err = disk.Dir(filepath.Join(bundle, ".next"), disk.Shared)
	if err != nil {
		return "", fmt.Errorf("create the bundle dir: %w", err)
	}

	copyCtx, cancelCopy := context.WithTimeout(ctx, copyTimeout)
	defer cancelCopy()

	copies := [][2]string{
		{filepath.Join(appDir, ".next", "standalone") + "/.", bundle},
		{filepath.Join(appDir, ".next", "static"), filepath.Join(bundle, ".next", "static")},
	}

	for _, pair := range copies {
		copyErr := exec.CommandContext(copyCtx, "cp", "-R", pair[0], pair[1]).Run()
		if copyErr != nil {
			return "", fmt.Errorf("assemble the bundle (%s): %w", pair[0], copyErr)
		}
	}

	return bundle, nil
}

// freePort asks the kernel for a loopback port nobody is using, so two
// suites running at once never collide.
func freePort(ctx context.Context) (int, error) {
	var config net.ListenConfig

	listener, err := config.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate a port: %w", err)
	}

	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("allocate a port: %w", errNotTCP)
	}

	return addr.Port, nil
}

// startServer spawns the bundle's own `node server.js` and waits for it
// to answer. Deliberately not CommandContext — see the nolint below: its
// lifetime is stop's, not this call's.
func (h *Harness) startServer(ctx context.Context, bundle string, port int) error {
	//nolint:noctx // the server outlives this call; stop() owns its lifetime
	server := exec.Command("node", "server.js")

	server.Dir = bundle

	server.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"HOSTNAME=127.0.0.1",
	)
	server.Stdout = console.Err()
	server.Stderr = console.Err()

	err := server.Start()
	if err != nil {
		return fmt.Errorf("start the application under test: %w", err)
	}

	h.server = server

	return h.waitForApp(ctx)
}

// waitForApp blocks until the application answers, or reports that it
// never did.
func (h *Harness) waitForApp(ctx context.Context) error {
	client := &http.Client{Timeout: probeTimeout}
	deadline := time.Now().Add(bootTimeout)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/", nil)
		if err != nil {
			return fmt.Errorf("build the readiness probe: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()

			return nil
		}

		time.Sleep(bootPollInterval)
	}

	return fmt.Errorf("%w: %s did not answer within %s", ErrServerNotReady, h.BaseURL, bootTimeout)
}

// openBrowser assembles the driver, makes sure chromium is present, and
// launches it.
func (h *Harness) openBrowser(ctx context.Context) error {
	driverDir, err := ensureDriver(ctx, h.RepoRoot)
	if err != nil {
		return err
	}

	options := &playwright.RunOptions{
		DriverDirectory: driverDir,
		Browsers:        []string{"chromium"},
	}

	err = playwright.Install(options)
	if err != nil {
		return fmt.Errorf("install the playwright driver: %w", err)
	}

	driver, err := playwright.Run(options)
	if err != nil {
		return fmt.Errorf("start playwright: %w", err)
	}

	h.driver = driver

	browser, err := driver.Chromium.Launch()
	if err != nil {
		return fmt.Errorf("launch chromium: %w", err)
	}

	h.Browser = browser

	return nil
}

// stop tears down everything NewHarness brought up, in reverse order.
// Every step is attempted even when an earlier one failed: a leaked node
// process outlives the test binary and holds its port.
func (h *Harness) stop() {
	if h.Browser != nil {
		_ = h.Browser.Close()
	}

	if h.driver != nil {
		_ = h.driver.Stop()
	}

	if h.server != nil && h.server.Process != nil {
		_ = h.server.Process.Kill()
		_, _ = h.server.Process.Wait()
	}
}

// Options builds the bddgo options for this suite.
func Options(repoRoot string) bddgo.Options {
	return bddgo.Options{
		Registry:     filepath.Join(repoRoot, "docs", "scenarios.yaml"),
		Architecture: filepath.Join(repoRoot, "docs", "architecture", "architecture.yaml"),
		Suite:        "bdd-web",
	}
}
