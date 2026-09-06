package steps

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// engineLogRel is where the engine writes its structured log, relative to the
// folder it runs in — the project tree, which is the remote's own working dir.
const engineLogRel = "tmp/true-bdd.log.json"

// registerAICallSteps binds the budget clause every AI scenario closes with.
func registerAICallSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the remote spawned at most (\d+) AI calls?$`, assertAICallCeiling)
}

// assertAICallCeiling holds the run to a ceiling on model turns, counted at the
// router's dispatch record so one number covers whichever CLI a tier named. An
// absent log reads as zero: a run ending before the engine booted spent nothing.
func assertAICallCeiling(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step names %q AI calls, which is not a number: %w", args[0], err)
	}

	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	path := filepath.Join(state.Tree.Dir, engineLogRel)

	got, err := countAIDispatches(state, path)
	if err != nil {
		return err
	}

	if got > want {
		return state.fail("the remote spawned %d AI call(s), want at most %d (%s)",
			got, want, path)
	}

	return nil
}

// countAIDispatches counts dispatch records in a newline-delimited slog file. A
// malformed line is skipped: the file is append-mode and only whole records
// matter here.
func countAIDispatches(state *State, path string) (int, error) {
	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}

	if err != nil {
		return 0, state.fail("reading the engine log %s: %w", path, err)
	}

	count := 0

	for _, line := range strings.Split(string(data), "\n") {
		var record struct {
			Msg string `json:"msg"`
		}

		if json.Unmarshal([]byte(strings.TrimSpace(line)), &record) != nil {
			continue
		}

		if record.Msg == enginelog.MsgDispatch {
			count++
		}
	}

	return count, nil
}
