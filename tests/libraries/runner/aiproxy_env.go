package runner

import "os"

// AI-proxy modes for the fixture suite. Live is today's behavior (real
// CLIs, no shim); record runs real CLIs through the aiproxy shim and
// writes cassettes; replay serves cassettes needing no real AI CLIs.
const (
	ProxyModeLive   = "live"
	ProxyModeRecord = "record"
	ProxyModeReplay = "replay"
)

// AIProxyEnv builds the env entries that activate the aiproxy shim for
// one fixture run (Execute's extraEnv, CLI subprocess only): PATH puts
// the shim first, and TRUE_BDD_AIPROXY_* is the shim's env contract.
func AIProxyEnv(mode, shimDir, cassettesDir, stateDir string) []string {
	return []string{
		"PATH=" + shimDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TRUE_BDD_AIPROXY_MODE=" + mode,
		"TRUE_BDD_AIPROXY_CASSETTES=" + cassettesDir,
		"TRUE_BDD_AIPROXY_STATE=" + stateDir,
		"TRUE_BDD_AIPROXY_DIR=" + shimDir,
	}
}
