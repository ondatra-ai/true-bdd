package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/fs"
)

// seedVersionManager builds a StoryVersionManager with one saved version
// so writeConvergedStory has a latest to load.
func seedVersionManager(t *testing.T) *fs.StoryVersionManager {
	t.Helper()

	runDir, err := fs.NewRunDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewRunDirectory: %v", err)
	}

	versionMgr := fs.NewStoryVersionManager(runDir, "4.1")

	err = versionMgr.SaveInitialVersion(&story.Story{ID: "4.1", Title: "Seed Story"})
	if err != nil {
		t.Fatalf("SaveInitialVersion: %v", err)
	}

	return versionMgr
}

// TestStoryFinalizeSurfacesWriteFailure proves the plan §3.2 change:
// a converged walk whose story file cannot be written no longer reports
// success. The finalize error propagates so the runner marks the result
// event finalization_ok=false and the CLI exits non-zero.
func TestStoryFinalizeSurfacesWriteFailure(t *testing.T) {
	t.Parallel()

	versionMgr := seedVersionManager(t)

	// A regular file occupies the parent of the stories dir, so
	// os.MkdirAll inside writeNewStoryFile fails deterministically.
	blocker := filepath.Join(t.TempDir(), "blocker")

	err := os.WriteFile(blocker, []byte("not a dir"), 0o644)
	if err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	storiesDir := filepath.Join(blocker, "stories")
	finalize := StoryFinalize(storiesDir, "4.1", versionMgr, true, true)

	err = finalize(&engine.Result[*story.Story]{Reason: engine.Converged})
	if err == nil {
		t.Fatal("expected finalize to surface the story-write failure")
	}
}

// TestStoryFinalizeConvergedWritesFile proves the happy path still writes
// the converged story and reports success.
func TestStoryFinalizeConvergedWritesFile(t *testing.T) {
	t.Parallel()

	versionMgr := seedVersionManager(t)
	storiesDir := filepath.Join(t.TempDir(), "stories")
	finalize := StoryFinalize(storiesDir, "4.1", versionMgr, true, true)

	err := finalize(&engine.Result[*story.Story]{Reason: engine.Converged})
	if err != nil {
		t.Fatalf("finalize returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(storiesDir, "4.1-*.yaml"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one written story file, got %v (err=%v)", matches, err)
	}
}

// TestStoryFinalizeNonConvergedIsClean proves non-converged stop reasons
// stay success-at-finalize (the write only happens on Converged).
func TestStoryFinalizeNonConvergedIsClean(t *testing.T) {
	t.Parallel()

	versionMgr := seedVersionManager(t)
	finalize := StoryFinalize(t.TempDir(), "4.1", versionMgr, true, true)

	err := finalize(&engine.Result[*story.Story]{Reason: engine.UserExit})
	if err != nil {
		t.Fatalf("user-exit finalize returned error: %v", err)
	}
}
