package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func registerWriterLifecycleSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the codex writer uses fixture-controlled authentication and CLIs$`, useFixtureWriter)
	suite.Step(`^the codex writer provisioned and removed its fixture authentication$`, assertWriterLifecycle)
}

//truebdd:concept given "fixture AI CLIs"
func useFixtureWriter(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("resolve writer fixture: %w", err)
	}

	if state.Harness.Modes.Services == runner.ProxyModeLive {
		run := runner.RunDir(state.Harness.SessionRoot, state.Fixture.Name)
		state.Proxy.StateDir = filepath.Join(run, "tmp", "aiproxy-state")
		state.Proxy.Env = runner.AIProxyEnv("observe", state.Harness.Shims.Services,
			fixture, state.Proxy.StateDir, state.Harness.Shims.All())
	}

	for index, entry := range state.Proxy.Env {
		if strings.HasPrefix(entry, "PATH=") {
			shim, rest, _ := strings.Cut(strings.TrimPrefix(entry, "PATH="), string(os.PathListSeparator))
			state.Proxy.Env[index] = "PATH=" + strings.Join(
				[]string{shim, filepath.Join(fixture, "cli"), rest}, string(os.PathListSeparator))
		}
	}

	state.Proxy.Env = append(state.Proxy.Env, "CODEX_HOME="+filepath.Join(fixture, "auth"),
		"TRUE_BDD_AIPROXY_WRITER_FIXTURE="+fixture)

	return nil
}

//truebdd:concept then "AI CLI observations"
func assertWriterLifecycle(state *State, _ []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	receipt, err := disk.Read(filepath.Join(state.Proxy.StateDir, "writer-spawn.txt"))
	if err != nil {
		return state.fail("writer spawn was not verified: %w", err)
	}

	if string(receipt) != "fixture authentication and edit roots verified\n" {
		return state.fail("unexpected writer spawn receipt")
	}

	homes, err := filepath.Glob(filepath.Join(state.Result.TmpDir, "tmp/codex-homes/*/*-codex-home"))
	if err != nil || len(homes) != 1 {
		return state.fail("expected exactly one generated writer home, got %d: %v", len(homes), err)
	}

	_, err = os.Lstat(filepath.Join(homes[0], "auth.json"))
	if !os.IsNotExist(err) {
		return state.fail("writer authentication remains after the turn: %v", err)
	}

	return nil
}
