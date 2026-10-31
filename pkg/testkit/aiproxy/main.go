// Command aiproxy is the record/replay PATH shim for the AI CLIs the
// true-bdd engine spawns (claude / codex). The BDD harness
// installs it under those names in a shim directory prepended to the
// engine subprocess's PATH — the engine is never modified and never
// knows which mode it runs under (design: tmp/ai-proxy-design.md).
//
// Modes (TRUE_BDD_AIPROXY_MODE):
//   - record: exec the real CLI (resolved from PATH minus the shim
//     dir), tee stdio through unmodified into a cassette, and
//     snapshot/diff the working tree around the call so file effects
//     are captured alongside the byte streams.
//   - observe: verify opted-in fixture spawns and pass through without recording.
//   - replay: never touch the real CLI — apply the cassette's fs-diff,
//     emit the recorded stdout verbatim, exit with the recorded code.
//
// Replay claims unconsumed cassettes by normalized request hash per binary.
// Identical requests consume occurrences in recorded-number order.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"log/slog"
)

var errUnknownMode = errors.New("unknown TRUE_BDD_AIPROXY_MODE")

func main() {
	logging.Install(logging.Stderr, "", "aiproxy")

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
	_, err := installedGoRoot()
	if err != nil {
		return 0, fmt.Errorf("resolve recording toolchain: %w", err)
	}

	err = verifyWriterSpawn(cfg, name)
	if err != nil {
		return 0, err
	}

	switch cfg.Mode {
	case "observe":
		path, err := resolveRealBinary(name, cfg.ShimDir)
		if err != nil {
			return 0, err
		}

		result, err := runReal(path, argv)
		if err != nil {
			return 0, err
		}

		return result.ExitCode, nil
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
	slog.Error("aiproxy failed", "error", err)
	os.Exit(exitProxyFailure)
}
