package steps

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"os"
	"path/filepath"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

func registerWriterLifecycleSteps(suite testkit.StepRegistrar[State]) {
	suite.Step(`^the codex writer uses fixture-controlled authentication and CLIs$`, useFixtureWriter)
	suite.Step(`^the codex writer started with private fixture authentication, `+
		`protected homes, an empty edit root and private shell scratch$`, assertWriterSpawn)
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

	state.ObserveServices(fixture)
	proxycli.FallbackPath(state, filepath.Join(fixture, "cli"))
	state.SetEnv("CODEX_HOME=" + filepath.Join(fixture, "auth"))
	homeRoot := state.WorkingDirectory()

	return proxycli.Observe(state, "codex", []proxycli.PathSpec{
		{Path: filepath.Join(fixture, "auth", "auth.json"), Contents: true, ContentRoot: fixture},
		{Path: "$CODEX_HOME"},
		{Path: "$CODEX_HOME/auth.json", Contents: true, ContentRoot: homeRoot},
		{Path: "$CODEX_HOME/config.toml", Contents: true, ContentRoot: homeRoot},
		{Path: "services/calc"},
		{Path: "tmp/codex-shell-home"}, {Path: "tmp/codex-tmp"}, {Path: "tmp/codex-go-build"},
	})
}

//truebdd:concept then "AI CLI observations"
func assertWriterLifecycle(state *State, _ []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
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
