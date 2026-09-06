// Package steps holds the bdd-web suite's step definitions: the Go code
// that executes the Given/When/Then lines of every registry scenario
// whose `service:` is bdd-web.
//
// Same runner as the CLI suite, different verbs. pkg/testkit/bddgo
// knows nothing about either — it binds a step's text to a function and
// hands that function whatever state the suite chose to keep. Here that
// state is a browser page; in tests/bdd-cli/steps it is a subprocess and
// a working tree. That is the whole of what "one framework, two
// surfaces" means in practice, and it is why a scenario can be written
// once and pointed at either.
//
// This is the first slice of the Go replacement for the parked
// Playwright suite this one replaced. It covers the
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
	"path/filepath"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/cli/npm"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
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
	// Port is the loopback port it answers on, kept so a restart comes back
	// where the remote already polling it is pointed.
	Port int
	// Bundle is the assembled standalone build every relay this suite starts
	// is served from, the harness's own included.
	Bundle string
	// Browser is shared; each scenario opens its own context, so cookies
	// and storage never leak from one scenario into the next.
	Browser playwright.Browser

	driver *playwright.Playwright
	server *cli.Process

	// cliBuild and materializerBuild are the two binaries a protocol
	// scenario needs, compiled at most once per `go test` invocation.
	cliBuild          buildOnce
	materializerBuild buildOnce
}

// NewHarness builds the application, starts it on a free loopback port,
// installs the Playwright driver, and launches a browser — once, cached
// under the user's home, which is why this suite skips the one-minute commit gate.
func NewHarness(ctx context.Context, mode, repoRoot string) (*Harness, func(), error) {
	bundle, err := buildBundle(repoRoot)
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
		Port:     port,
		Bundle:   bundle,
	}

	err = harness.startServer(ctx, bundle, port)
	if err != nil {
		harness.stop()

		return nil, nil, err
	}

	err = harness.openBrowser()
	if err != nil {
		harness.stop()

		return nil, nil, err
	}

	return harness, harness.stop, nil
}

// buildOnce is a binary this suite compiles at most once, remembering the
// failure too: a second asker is told the same thing rather than
// rebuilding into the same error.
type buildOnce struct {
	once sync.Once
	path string
	err  error
}

// resolve builds pkg into tmp/bdd-web-bin/<binName> on the first ask.
func (b *buildOnce) resolve(repoRoot, binName, pkg string) (string, error) {
	b.once.Do(func() {
		dir := filepath.Join(repoRoot, "tmp", "bdd-web-bin")

		err := disk.Dir(dir, disk.Shared)
		if err != nil {
			b.err = fmt.Errorf("create the bin dir: %w", err)

			return
		}

		path := filepath.Join(dir, binName)

		err = gotool.Build(cli.Options{Timeout: buildTimeout}, repoRoot, path, pkg)
		if err != nil {
			b.err = fmt.Errorf("build %s: %w", pkg, err)

			return
		}

		b.path = path
	})

	return b.path, b.err
}

// CLIBinary is the true-bdd a remote is spawned from.
func (h *Harness) CLIBinary() (string, error) {
	return h.cliBuild.resolve(h.RepoRoot, "true-bdd", "./services/bdd-cli")
}

// MaterializerBinary is the shared fixture materializer this suite shells
// to rather than re-implementing overlay, checklist-filter and prep
// semantics; its doc.go states the CLI contract.
func (h *Harness) MaterializerBinary() (string, error) {
	return h.materializerBuild.resolve(h.RepoRoot, "materializer",
		"./pkg/testkit/materializer")
}

// buildBundle compiles the app and assembles the standalone bundle
// exactly as the Dockerfile's runtime stage does. Built from source every
// time, not cached: a stale build would make this suite's green mean nothing.
func buildBundle(repoRoot string) (string, error) {
	appDir := filepath.Join(repoRoot, "services", "bdd-web")

	built, err := npm.RunScript("build", cli.Options{
		Timeout: buildTimeout,
		Dir:     appDir,
		Output:  cli.Streams(console.Err(), console.Err()),
	})
	if err == nil {
		err = built.Err()
	}

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

	// dst, src: CopyTree lands src's CONTENTS at dst, which is what the
	// `cp -R standalone/. <bundle>` this replaced meant.
	copies := [][2]string{
		{bundle, filepath.Join(appDir, ".next", "standalone")},
		{filepath.Join(bundle, ".next", "static"), filepath.Join(appDir, ".next", "static")},
	}

	for _, pair := range copies {
		copyErr := disk.CopyTree(pair[0], pair[1], disk.Shared)
		if copyErr != nil {
			return "", fmt.Errorf("assemble the bundle (%s): %w", pair[1], copyErr)
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

// Restart kills the shared relay and starts another on the same port, so a
// scenario about restart survival keeps the URL its remote is polling.
func (h *Harness) Restart(ctx context.Context) error {
	if h.server != nil {
		_ = h.server.Signal(os.Kill)
		_, _ = h.server.Wait()
	}

	return h.startServer(ctx, h.Bundle, h.Port)
}

// startServer spawns the bundle's own `node server.js` and waits for it
// to answer.
func (h *Harness) startServer(ctx context.Context, bundle string, port int) error {
	server, err := startNodeServer(ctx, bundle, port, nil)
	if err != nil {
		return err
	}

	h.server = server

	return waitForRelay(ctx, h.BaseURL)
}

// startNodeServer spawns one relay process on port, under the settings a
// caller adds to the two every relay needs. Deliberately not CommandContext:
// the server outlives this call, and the caller's teardown owns it.
func startNodeServer(ctx context.Context, bundle string, port int, env []string) (*cli.Process, error) {
	settings := append([]string{fmt.Sprintf("PORT=%d", port), "HOSTNAME=127.0.0.1"}, env...)

	server, err := npm.StartNode(context.WithoutCancel(ctx), []string{"server.js"}, cli.Options{
		Dir:    bundle,
		Env:    cli.Inherit().Set(settings...),
		Output: cli.Streams(console.Err(), console.Err()),
	})
	if err != nil {
		return nil, fmt.Errorf("start the application under test: %w", err)
	}

	return server, nil
}

// waitForRelay blocks until the relay at baseURL answers, or reports that it
// never did.
func waitForRelay(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: probeTimeout}
	deadline := time.Now().Add(bootTimeout)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
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

	return fmt.Errorf("%w: %s did not answer within %s", ErrServerNotReady, baseURL, bootTimeout)
}

// openBrowser assembles the driver, makes sure chromium is present, and
// launches it.
func (h *Harness) openBrowser() error {
	driverDir, err := ensureDriver(h.RepoRoot)
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

	if h.server != nil {
		_ = h.server.Signal(os.Kill)
		_, _ = h.server.Wait()
	}
}

// Options builds the bddgo options for this suite.
func Options(repoRoot string) bddgo.Options {
	return bddgo.Options{
		Registry:     filepath.Join(repoRoot, "docs", "scenarios.yaml"),
		Architecture: filepath.Join(repoRoot, "docs", architectureNode, "architecture.yaml"),
		Suite:        "bdd-web",
	}
}
