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

	// testChecklistName is the stem validateFixTemplates must name in
	// its refusal, so the reader knows which YAML to open.
	testChecklistName = "us-create"
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
// finalize's error now propagates instead of being swallowed, so a
// converged walk whose story never landed no longer reports success.
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
// inherits and validates the checklist's default_docs too.
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

// promptFixing builds a flattened prompt in the shape
// validateFixTemplates reads: the section it sits under, the question it
// asks, and the F: template that would guide its fix.
func promptFixing(section, question, fixTemplate string) checklistmodels.PromptWithContext {
	return checklistmodels.PromptWithContext{
		SectionID:   testChecklistName,
		CriterionID: section,
		Prompt: checklistmodels.Prompt{
			Question:    question,
			FixTemplate: fixTemplate,
		},
	}
}

// TestValidateFixTemplatesAcceptsFullyFixableChecklist is the green
// path: every prompt carries an F:, so --fix can apply at every cell.
// us-apply, build-code and build-tests all ship this shape.
func TestValidateFixTemplatesAcceptsFullyFixableChecklist(t *testing.T) {
	err := validateFixTemplates(true, testChecklistName, []checklistmodels.PromptWithContext{
		promptFixing("format", "Does the story follow the format?", "Rewrite it."),
		promptFixing("why", "Does the so_that clause describe a benefit?", "State the benefit."),
	})
	if err != nil {
		t.Fatalf("validateFixTemplates: unexpected error %v", err)
	}
}

// TestValidateFixTemplatesRejectsMissingFixTemplate is the regression
// this check exists for: without an F: the fix turn would run with
// nothing to say, so the refusal must name the checklist and prompt.
func TestValidateFixTemplatesRejectsMissingFixTemplate(t *testing.T) {
	err := validateFixTemplates(true, testChecklistName, []checklistmodels.PromptWithContext{
		promptFixing("who", "Does the as_a clause use a known role?\nRead the product doc.", ""),
	})
	if !errors.Is(err, errChecklistNotFixable) {
		t.Fatalf("error = %v, want errChecklistNotFixable", err)
	}

	for _, want := range []string{
		testChecklistName,
		"us-create/who #1",
		"Does the as_a clause use a known role?",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got %q", want, err)
		}
	}
}

// TestValidateFixTemplatesReportsAllMissing proves one run names every
// unfixable prompt rather than stopping at the first, and leaves the
// fixable one out of the list.
func TestValidateFixTemplatesReportsAllMissing(t *testing.T) {
	err := validateFixTemplates(true, testChecklistName, []checklistmodels.PromptWithContext{
		promptFixing("format", "Q one", ""),
		promptFixing("format", "Q two", "Fix it."),
		promptFixing("why", "Q three", ""),
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	for _, want := range []string{"us-create/format #1", "us-create/why #3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got %q", want, err)
		}
	}

	if strings.Contains(err.Error(), "Q two") {
		t.Errorf("refusal must not name the fixable prompt, got %q", err)
	}
}

// TestValidateFixTemplatesTreatsBlankTemplateAsMissing closes the hole a
// present-but-empty F: would leave: it renders nothing into the prompt,
// so it is absent for every purpose that matters.
func TestValidateFixTemplatesTreatsBlankTemplateAsMissing(t *testing.T) {
	err := validateFixTemplates(true, testChecklistName, []checklistmodels.PromptWithContext{
		promptFixing("what", "Q one", "  \n\t "),
	})
	if !errors.Is(err, errChecklistNotFixable) {
		t.Fatalf("a blank F: must be treated as missing; error = %v", err)
	}
}

// TestValidateFixTemplatesAllowsEmptyPromptList keeps the check inert
// when there is nothing to walk — a narrowed or empty checklist is a
// different refusal's business, not this one's.
func TestValidateFixTemplatesAllowsEmptyPromptList(t *testing.T) {
	err := validateFixTemplates(true, testChecklistName, nil)
	if err != nil {
		t.Fatalf("an empty prompt list must pass, got %v", err)
	}
}

// TestValidateFixTemplatesIsInertWithoutTheFlag is the other half of
// the contract: a checklist with no F: anywhere still walks fine
// without --fix — us-create/us-refine are exactly this shape.
func TestValidateFixTemplatesIsInertWithoutTheFlag(t *testing.T) {
	err := validateFixTemplates(false, testChecklistName, []checklistmodels.PromptWithContext{
		promptFixing("format", "Q one", ""),
		promptFixing("why", "Q two", ""),
	})
	if err != nil {
		t.Fatalf("a walk without --fix must not be refused, got %v", err)
	}
}
