package steps

import (
	"path/filepath"
	"strconv"
	"strings"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

const privateWriterFile = 0o600
const privateWriterDirectory = 0o700
const initialWriterHomeFiles = 2
const dummyWriterAuth = "{\"true_bdd_dummy_auth\":\"issue-210-no-credentials\"}\n"

//truebdd:concept then "AI CLI observations"
func assertWriterSpawn(state *State, _ []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	spawns, err := proxycli.Spawns(state)
	if err != nil {
		return state.fail("read writer spawn: %w", err)
	}

	if len(spawns) != 1 || spawns[0].Binary != "codex" {
		return state.fail("expected exactly one codex writer spawn, got %d", len(spawns))
	}

	spawn := spawns[0]

	root, err := filepath.EvalSymlinks(state.Result.TmpDir)
	if err != nil {
		return state.fail("resolve writer workspace: %w", err)
	}

	err = checkWriterHome(state, spawn.Paths["$CODEX_HOME"], root)
	if err != nil {
		return err
	}

	err = checkWriterAuthentication(state, spawn)
	if err != nil {
		return err
	}

	return checkWriterRoots(state, spawn, root)
}

func checkWriterAuthentication(state *State, spawn proxycli.Spawn) error {
	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("resolve fixture authentication: %w", err)
	}

	source := spawn.Paths[filepath.Join(fixture, "auth", "auth.json")]
	if source.Error != "" || string(source.Content) != dummyWriterAuth {
		return state.fail("fixture must contain only declared dummy authentication")
	}

	copied := spawn.Paths["$CODEX_HOME/auth.json"]

	err = checkPrivateWriterPath(state, "copied authentication", copied, false)
	if err != nil {
		return err
	}

	if string(copied.Content) != string(source.Content) {
		return state.fail("authentication differs from dummy fixture")
	}

	return nil
}

func checkWriterRoots(state *State, spawn proxycli.Spawn, root string) error {
	edit := spawn.Paths["services/calc"]
	if edit.Error != "" || edit.Type != "directory" || len(edit.Entries) != 0 {
		return state.fail("edit root must exist and be empty before controlled writer")
	}

	for _, name := range []string{"tmp/codex-shell-home", "tmp/codex-tmp", "tmp/codex-go-build"} {
		err := checkPrivateWriterPath(state, name, spawn.Paths[name], true)
		if err != nil {
			return err
		}
	}

	config := spawn.Paths["$CODEX_HOME/config.toml"]

	err := checkPrivateWriterPath(state, "generated configuration", config, false)
	if err != nil {
		return err
	}

	denial := strconv.Quote(filepath.Join(root, "tmp", "codex-homes")) + " = \"deny\"\n"
	if !strings.Contains(string(config.Content), denial) {
		return state.fail("generated homes are not protected by the writer configuration")
	}

	return nil
}

func checkPrivateWriterPath(state *State, label string, observed proxycli.PathObservation, directory bool) error {
	wantMode, wantType := uint32(privateWriterFile), "file"
	if directory {
		wantMode, wantType = privateWriterDirectory, "directory"
	}

	if observed.Error != "" || observed.Symlink || observed.Mode != wantMode || observed.Type != wantType {
		return state.fail("%s has wrong type or permissions at spawn: %+v", label, observed)
	}

	return nil
}

func checkWriterHome(state *State, home proxycli.PathObservation, root string) error {
	expected := filepath.Join(root, "tmp", "codex-homes") + string(filepath.Separator)
	if !strings.HasPrefix(home.Canonical, expected) || !strings.HasSuffix(home.Canonical, "-codex-home") {
		return state.fail("generated CODEX_HOME is not protected beneath the host's private homes")
	}

	err := checkPrivateWriterPath(state, "generated home", home, true)
	if err != nil {
		return err
	}

	if len(home.Entries) != initialWriterHomeFiles {
		return state.fail("generated home must contain only auth.json and config.toml at spawn")
	}

	return nil
}
