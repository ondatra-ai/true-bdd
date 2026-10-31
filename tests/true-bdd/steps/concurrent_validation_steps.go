package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func registerConcurrentValidationSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^Scenario validation overlaps with ordered gates and fix menus$`, assertConcurrentValidation)
	suite.Step(`^replay consumes the recorded validation requests in reverse order$`, assertReverseRequests)
	suite.Step(`^the validation CLIs are controlled for concurrent arrival$`, controlConcurrentArrival)
}

//truebdd:concept given "fixture AI CLIs"
func controlConcurrentArrival(state *State, _ []string) error {
	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("resolve validation fixture: %w", err)
	}

	if state.Harness.Modes.Services == runner.ProxyModeLive {
		run := runner.RunDir(state.Harness.SessionRoot, state.Fixture.Name)
		state.Proxy.StateDir = filepath.Join(run, "tmp", "aiproxy-state")
		state.Proxy.Env = runner.AIProxyEnv("observe", state.Harness.Shims.Services,
			fixture, state.Proxy.StateDir, state.Harness.Shims.All())
	}

	for index, entry := range state.Proxy.Env {
		if !strings.HasPrefix(entry, "PATH=") {
			continue
		}

		path := strings.TrimPrefix(entry, "PATH=")
		shim, _, _ := strings.Cut(path, string(os.PathListSeparator))
		dirs := []string{filepath.Join(fixture, "front"), shim, filepath.Join(fixture, "cli"), path}
		state.Proxy.Env[index] = "PATH=" + strings.Join(dirs, string(os.PathListSeparator))
		state.Proxy.Env = append(state.Proxy.Env, "TRUE_BDD_CONCURRENT_PROXY="+filepath.Join(shim, "claude"))

		break
	}

	return nil
}

//truebdd:concept then "engine activity"
func assertConcurrentValidation(state *State, _ []string) error {
	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	observed := newValidationObservation()
	for _, record := range records {
		err = observed.observe(state, record)
		if err != nil {
			return err
		}
	}

	if !observed.overlap || strings.Join(observed.menus, ",") != "alpha,beta" {
		return state.fail("overlap=%v, applied menus=%v", observed.overlap, observed.menus)
	}

	return nil
}

//truebdd:concept then "AI CLI observations"
func assertReverseRequests(state *State, _ []string) error {
	if state.Harness.Modes.Services != runner.ProxyModeReplay {
		return nil
	}

	data, err := disk.Read(filepath.Join(state.Proxy.StateDir, "consumed-claude"))
	if err != nil {
		return state.fail("read request claims: %w", err)
	}

	if !strings.HasPrefix(string(data), `["claude-002"`) {
		return state.fail("reverse replay did not consume second recorded request first: %s", data)
	}

	return nil
}
