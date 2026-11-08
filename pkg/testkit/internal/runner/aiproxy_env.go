package runner

import (
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"os"
	"strings"
)

// AI-proxy modes for the fixture suite. Live is today's behavior (real
// CLIs, no shim); record runs real CLIs through the aiproxy shim and
// writes cassettes; replay serves cassettes needing no real AI CLIs.
const (
	ProxyModeLive   = "live"
	ProxyModeRecord = "record"
	ProxyModeReplay = "replay"
)

// EnvKnownShims lists every shim dir this run installed. A recording
// shim skips all of them when hunting the genuine CLI: skipping only its
// own finds the other caller's shim and execs it, which recurses.
const EnvKnownShims = "TRUE_BDD_AIPROXY_KNOWN_SHIMS"

// AIProxyEnv activates the aiproxy shim for one caller: its own shim
// leads PATH, TRUE_BDD_AIPROXY_* is the shim's contract, and every other
// caller's shim is scrubbed — else its turns answer from the wrong shelf.
func AIProxyEnv(mode, shimDir, cassettesDir, stateDir string, knownShims []string) []string {
	return proxycli.Environment(mode, shimDir, cassettesDir, stateDir, knownShims)
}

// ShimDirs is one run's shim directory per caller. Two dirs, not one:
// the shim reads its whole configuration from flat env vars, so the only
// way to give two callers different modes is to give them different dirs.
type ShimDirs struct {
	Services string
	Tests    string
}

// All is every installed shim dir, for the known-shims contract.
func (d ShimDirs) All() []string {
	dirs := make([]string, 0, 2) //nolint:mnd // one dir per caller.

	for _, dir := range []string{d.Services, d.Tests} {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}

	return dirs
}

// ArmProcess applies env entries to THIS process and returns a function
// restoring what was there. Scoped: the engine subprocess inherits this
// environment, so a shelf left armed answers a services turn from it.
func ArmProcess(entries []string) func() {
	undo := make([]func(), 0, len(entries))

	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}

		previous, had := os.LookupEnv(key)

		undo = append(undo, func() {
			if had {
				_ = os.Setenv(key, previous)

				return
			}

			_ = os.Unsetenv(key)
		})

		_ = os.Setenv(key, value)
	}

	return func() {
		for index := len(undo) - 1; index >= 0; index-- {
			undo[index]()
		}
	}
}
