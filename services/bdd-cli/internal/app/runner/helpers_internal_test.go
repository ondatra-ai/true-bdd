package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
)

const (
	testProductRel  = "docs/product/product.yaml"
	testProductPath = "./" + testProductRel
	testArchPath    = "./docs/architecture/architecture.yaml"
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

// newDocResolver builds a Resolver over a throwaway project. Mirrors
// the docs package's own helper; kept local so this package's tests do
// not depend on an exported test fixture.
func newDocResolver(t *testing.T, documents map[string]string, present []string) *docs.Resolver {
	t.Helper()

	root := t.TempDir()

	var body strings.Builder

	body.WriteString("documents:\n")

	for key, path := range documents {
		body.WriteString("  " + key + ": \"" + path + "\"\n")
	}

	writeTestFile(t, filepath.Join(root, "true-bdd", "true-bdd.yaml"), body.String())

	for _, rel := range present {
		writeTestFile(t, filepath.Join(root, rel), "content\n")
	}

	t.Chdir(root)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatalf("NewViperConfig: %v", err)
	}

	return docs.NewResolver(cfg)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}

	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// promptDeclaring builds a flattened prompt carrying the given
// per-prompt docs and checklist-level default docs.
func promptDeclaring(promptDocs, defaultDocs []string) checklistmodels.PromptWithContext {
	return checklistmodels.PromptWithContext{
		DefaultDocs: defaultDocs,
		Prompt:      checklistmodels.Prompt{Question: "q?", Docs: promptDocs},
	}
}

// TestValidateRequiredDocsAcceptsSatisfiedChecklist is the green path:
// every declared document exists, so the walk is allowed to start.
func TestValidateRequiredDocsAcceptsSatisfiedChecklist(t *testing.T) {
	resolver := newDocResolver(t,
		map[string]string{docs.KeyProduct: testProductPath},
		[]string{testProductRel},
	)

	err := validateRequiredDocs(
		[]checklistmodels.PromptWithContext{promptDeclaring([]string{docs.KeyProduct}, nil)},
		resolver,
	)
	if err != nil {
		t.Fatalf("validateRequiredDocs: unexpected error %v", err)
	}
}

// TestValidateRequiredDocsRejectsMissingDocument is the regression
// this whole change exists for: a prompt declaring a document that is
// not on disk must stop the run rather than silently degrade it.
func TestValidateRequiredDocsRejectsMissingDocument(t *testing.T) {
	resolver := newDocResolver(t,
		map[string]string{docs.KeyArchitectureYAML: testArchPath},
		nil,
	)

	err := validateRequiredDocs(
		[]checklistmodels.PromptWithContext{
			promptDeclaring([]string{docs.KeyArchitectureYAML}, nil),
		},
		resolver,
	)
	if !errors.Is(err, errUnsatisfiableChecklistDocs) {
		t.Fatalf("error = %v, want errUnsatisfiableChecklistDocs", err)
	}

	if !strings.Contains(err.Error(), testArchPath) {
		t.Errorf("error must name the missing document, got %q", err)
	}
}

// TestValidateRequiredDocsReportsAllMissing proves one run names every
// unsatisfiable document, not just the first.
func TestValidateRequiredDocsReportsAllMissing(t *testing.T) {
	resolver := newDocResolver(t, map[string]string{
		docs.KeyProduct:          testProductPath,
		docs.KeyArchitectureYAML: testArchPath,
	}, nil)

	err := validateRequiredDocs([]checklistmodels.PromptWithContext{
		promptDeclaring([]string{docs.KeyProduct}, nil),
		promptDeclaring([]string{docs.KeyArchitectureYAML}, nil),
	}, resolver)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	for _, want := range []string{testProductPath, testArchPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name %s, got %q", want, err)
		}
	}
}

// TestValidateRequiredDocsHonoursDefaultDocs covers the fallback
// GetEffectiveDocs implements: a prompt with no docs: of its own
// inherits the checklist's default_docs, and those must be validated
// too.
func TestValidateRequiredDocsHonoursDefaultDocs(t *testing.T) {
	resolver := newDocResolver(t,
		map[string]string{docs.KeyProduct: testProductPath},
		nil,
	)

	err := validateRequiredDocs(
		[]checklistmodels.PromptWithContext{promptDeclaring(nil, []string{docs.KeyProduct})},
		resolver,
	)
	if !errors.Is(err, errUnsatisfiableChecklistDocs) {
		t.Fatalf("default_docs must be validated; error = %v", err)
	}
}

// TestValidateRequiredDocsRejectsUnknownKey treats a typo in a
// checklist as the startup error it is.
func TestValidateRequiredDocsRejectsUnknownKey(t *testing.T) {
	resolver := newDocResolver(t,
		map[string]string{docs.KeyProduct: testProductPath},
		[]string{testProductRel},
	)

	err := validateRequiredDocs(
		[]checklistmodels.PromptWithContext{promptDeclaring([]string{"typo_doc"}, nil)},
		resolver,
	)
	if !errors.Is(err, errUnsatisfiableChecklistDocs) {
		t.Fatalf("error = %v, want errUnsatisfiableChecklistDocs", err)
	}
}

// TestValidateRequiredDocsAllowsChecklistWithoutDocs keeps the check
// inert for checklists that declare none — us-apply and build-tests
// ship exactly that shape.
func TestValidateRequiredDocsAllowsChecklistWithoutDocs(t *testing.T) {
	resolver := newDocResolver(t, map[string]string{docs.KeyProduct: testProductPath}, nil)

	err := validateRequiredDocs(
		[]checklistmodels.PromptWithContext{promptDeclaring(nil, nil)},
		resolver,
	)
	if err != nil {
		t.Fatalf("a checklist declaring no docs must pass, got %v", err)
	}
}
