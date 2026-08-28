// Command report-server serves the BDD run report over HTTP.
//
//	go run ./tests/libraries/cmd/report-server
//
// It reads every session under tmp/test_run, rescans on an interval, and
// serves a UI that navigates between runs, compares any two of them, and
// diffs one test turn by turn.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/tests/libraries/reportserver"
)

// Defaults. The address is loopback on purpose: these responses carry
// whole prompt bodies and project source, and this is a developer tool,
// not a service.
const (
	defaultAddr         = "127.0.0.1:7331"
	readHeaderTimeout   = 10 * time.Second
	shutdownGracePeriod = 5 * time.Second
)

func main() {
	logging.Install(logging.Stderr, "", "report-server")

	err := run()
	if err != nil {
		slog.Error("report-server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", defaultAddr, "address to listen on")
	runsDir := flag.String("runs", "", "runs directory (default tmp/test_run under the repo root)")
	interval := flag.Duration("interval", reportserver.DefaultInterval, "how often to rescan the runs directory")

	flag.Parse()

	store, err := reportserver.NewStore(*runsDir, *interval)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// Scan once before listening, so the first request is fast and a
	// broken runs directory is reported here rather than as a 500.
	snapshot, err := store.Refresh()
	if err != nil {
		return fmt.Errorf("initial scan: %w", err)
	}

	slog.Info("report server ready",
		"runs", len(snapshot.Runs),
		"dir", store.RunsDir(),
		"url", "http://"+*addr,
		"rescan", interval.String())

	server := &http.Server{
		Addr:              *addr,
		Handler:           reportserver.NewServer(store).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return serve(server)
}

// serve runs the server until interrupted, then drains in-flight
// requests rather than cutting them off.
func serve(server *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errs := make(chan error, 1)

	go func() {
		listenErr := server.ListenAndServe()
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errs <- listenErr

			return
		}

		errs <- nil
	}()

	select {
	case err := <-errs:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}

		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}
