package steps

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func registerIOAtomicSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the System Architect runs "([^"]+)" after planting a stale staging symlink$`, runWithStagingSymlink)
	suite.Step(`^the project tree holds atomic replacement probes$`, prepareAtomicProbes)
	suite.Step(`^the rewritten file "([^"]+)" keeps private permissions and replaces its old hard link "([^"]+)"$`,
		assertAtomicReplacement)
}

//truebdd:concept then "file content"
func assertAtomicReplacement(state *State, args []string) error {
	target := filepath.Join(state.Result.TmpDir, args[0])
	original := filepath.Join(state.Result.TmpDir, args[1])

	current, err := os.Stat(target)
	if err != nil {
		return state.fail("stat rewritten file: %w", err)
	}

	before, err := os.Stat(original)
	if err != nil {
		return state.fail("stat original hard link: %w", err)
	}

	if current.Mode().Perm() != 0o600 || os.SameFile(current, before) {
		return state.fail("rewritten file must retain mode 0600 and have a new inode")
	}

	now, err := disk.Read(target)
	if err != nil {
		return state.fail("read rewritten file: %w", err)
	}

	old, err := disk.Read(original)
	if err != nil {
		return state.fail("read original hard link: %w", err)
	}

	if bytes.Equal(now, old) {
		return state.fail("original hard link was rewritten with the target")
	}

	return nil
}

//truebdd:concept given "project tree"
func prepareAtomicProbes(state *State, _ []string) error {
	if state.Fixture == nil || len(state.Fixture.PrepCmds) == 0 {
		return state.fail("atomic replacement probes require fixture preparation")
	}

	return nil
}

//truebdd:concept when "a command runs under interference"
func runWithStagingSymlink(state *State, args []string) error {
	return runObservedIO(state, args[0], "yes", nil, func(root string, _ <-chan struct{}) error {
		err := awaitIOFile(filepath.Join(root, "tmp/hold-ready"))
		if err != nil {
			return state.fail("staging launch barrier: %w", err)
		}

		err = os.Link(filepath.Join(root, "tests/mcp/ticket_test.go"), filepath.Join(root, "tmp/original.go"))
		if err != nil {
			return state.fail("preserve old hard link: %w", err)
		}

		err = os.Symlink("../../tmp/staging-target.txt", filepath.Join(root, "tests/mcp/.true-bdd.tmp.ticket_test.go"))
		if err != nil {
			return state.fail("plant staging symlink: %w", err)
		}

		err = disk.Write(filepath.Join(root, "tmp/hold-continue"), []byte("continue\n"), disk.Shared)
		if err != nil {
			return state.fail("release staging barrier: %w", err)
		}

		return nil
	})
}
