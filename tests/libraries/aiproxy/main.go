// Command aiproxy is the record/replay PATH shim for the AI CLIs the
// true-bdd engine spawns (claude / crush / codex). The BDD harness
// installs it under those names in a shim directory prepended to the
// engine subprocess's PATH — the engine is never modified and never
// knows which mode it runs under (design: tmp/ai-proxy-design.md).
//
// Modes (TRUE_BDD_AIPROXY_MODE):
//   - record: exec the real CLI (resolved from PATH minus the shim
//     dir), tee stdio through unmodified into a cassette, and
//     snapshot/diff the working tree around the call so file effects
//     are captured alongside the byte streams.
//   - replay: never touch the real CLI — apply the cassette's fs-diff,
//     emit the recorded stdout verbatim, exit with the recorded code.
//
// Cassettes are matched by arrival order per binary name (claude-001,
// claude-002, crush-001, …); a normalized request hash verifies each
// match and fails loudly on drift instead of replaying garbage.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errUnknownMode = errors.New("unknown TRUE_BDD_AIPROXY_MODE")

func main() {
	name := filepath.Base(os.Args[0])

	cfg, err := loadConfig()
	if err != nil {
		fail(err)
	}

	code, err := run(cfg, name, os.Args[1:])
	if err != nil {
		fail(err)
	}

	os.Exit(code)
}

func run(cfg config, name string, argv []string) (int, error) {
	switch cfg.Mode {
	case modeRecord:
		return record(cfg, name, argv)
	case modeReplay:
		return replay(cfg, name, argv)
	default:
		return 0, fmt.Errorf("%w: %q", errUnknownMode, cfg.Mode)
	}
}

// fail reports a proxy-level failure on stderr and exits with the
// distinctive proxy code — the message lands in the engine's combined CLI
// transcript, which is where a failed fixture gets read.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "aiproxy:", err)
	os.Exit(exitProxyFailure)
}
