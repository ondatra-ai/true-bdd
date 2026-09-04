package main

import (
	"errors"
	"fmt"
	"os"
)

const (
	envMode      = "TRUE_BDD_AIPROXY_MODE"
	envCassettes = "TRUE_BDD_AIPROXY_CASSETTES"
	envState     = "TRUE_BDD_AIPROXY_STATE"
	envShimDir   = "TRUE_BDD_AIPROXY_DIR"
	// envKnownShims lists every shim dir the run installed, so a recording
	// shim skips all of them rather than only its own.
	envKnownShims = "TRUE_BDD_AIPROXY_KNOWN_SHIMS"

	modeRecord = "record"
	modeReplay = "replay"

	// exitProxyFailure is the shim's own distinctive failure code, so a
	// missing/stale cassette is never confused with the wrapped CLI's exit codes.
	exitProxyFailure = 86
)

var errMissingEnv = errors.New("required environment variable missing")

// config is the shim's env contract, set by the BDD harness on the
// true-bdd subprocess (runner.AIProxyEnv) and inherited by every AI-CLI
// child the engine spawns.
type config struct {
	Mode      string // record | replay
	Cassettes string // cassette directory (per fixture)
	StateDir  string // per-run scratch for the call cursors
	ShimDir   string // the shim's own dir, skipped when resolving the real CLI
}

func loadConfig() (config, error) {
	cfg := config{
		Mode:      os.Getenv(envMode),
		Cassettes: os.Getenv(envCassettes),
		StateDir:  os.Getenv(envState),
		ShimDir:   os.Getenv(envShimDir),
	}

	for name, value := range map[string]string{
		envMode:      cfg.Mode,
		envCassettes: cfg.Cassettes,
		envState:     cfg.StateDir,
		envShimDir:   cfg.ShimDir,
	} {
		if value == "" {
			return config{}, fmt.Errorf("%w: %s", errMissingEnv, name)
		}
	}

	return cfg, nil
}
