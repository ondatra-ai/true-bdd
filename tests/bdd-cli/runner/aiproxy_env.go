package runner

import "os"

// AI-proxy modes for the fixture suite. Live is today's behavior (real
// CLIs, no shim); record runs the real CLIs through the aiproxy shim
// and writes cassettes; replay serves cassettes and needs no real AI
// CLIs. See tmp/ai-proxy-design.md and tests/aiproxy.
const (
	ProxyModeLive   = "live"
	ProxyModeRecord = "record"
	ProxyModeReplay = "replay"
)

// AIProxyEnv builds the env entries that activate the aiproxy shim for
// one fixture run. Passed as Execute's extraEnv, they apply to the CLI
// subprocess alone: the PATH override makes the engine's LookPath
// resolve claude/crush/codex to the shim, and the TRUE_BDD_AIPROXY_*
// vars are the shim's contract (mode, cassette dir, cursor state dir,
// and its own dir so record mode can resolve the real binary by
// skipping it on PATH).
func AIProxyEnv(mode, shimDir, cassettesDir, stateDir string) []string {
	return []string{
		"PATH=" + shimDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TRUE_BDD_AIPROXY_MODE=" + mode,
		"TRUE_BDD_AIPROXY_CASSETTES=" + cassettesDir,
		"TRUE_BDD_AIPROXY_STATE=" + stateDir,
		"TRUE_BDD_AIPROXY_DIR=" + shimDir,
	}
}
