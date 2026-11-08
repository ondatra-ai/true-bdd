package runner_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
)

type storyFixAnswer struct {
	requests *[]string
	applies  *int
	key      string
}

func (answer storyFixAnswer) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions) (string, error) {
	if role == provider.RoleFix {
		*answer.requests = append(*answer.requests, user)

		return `{"fix_prompt":"Write the summary steps","questions":[]}`, nil
	}

	*answer.applies++

	body := "title: Repaired\nacceptance_criteria:\n- id: 001-summary\n  steps:\n" +
		"  - then: [The summary is displayed]\n  - " + answer.key + ": The limit is displayed\n"
	if *answer.applies > 1 {
		body = "title: Repaired\nacceptance_criteria:\n- id: 001-summary\n  steps:\n" +
			"  - then:\n    - The summary is displayed\n    - and: The limit is displayed\n"
	}

	return "=== FILE_START: " + user + " ===\n" + body + "=== FILE_END: " + user + " ===", nil
}

func TestStoryFixRefusalReachesNextProposal(t *testing.T) {
	for _, key := range []string{"and", "but", "whenever"} {
		t.Run(key, func(t *testing.T) { testStoryFixRefusal(t, key) })
	}
}

func testStoryFixRefusal(t *testing.T, key string) {
	t.Helper()

	requests := make([]string, 0)
	applies := 0
	spec := notTakenSpec(t, storyFixAnswer{requests: &requests, applies: &applies, key: key})

	runDir, err := fs.NewRunDirectory(filepath.Join(spec.TmpDir, "versions"))
	if err != nil {
		t.Fatal(err)
	}

	versions := fs.NewStoryVersionManager(runDir, "summary")

	initial := &story.Story{ID: "summary", Title: "Original", Feature: "documents"}

	err = versions.SaveInitialVersion(initial)
	if err != nil {
		t.Fatal(err)
	}

	postFix := runner.StoryPostFix(versions)

	spec.PostFix = func(ctx context.Context, item bool, content string) (bool, error) {
		next, postErr := postFix(ctx, initial, content)
		if applies == 1 {
			assertStoryFixNotTaken(t, versions, postErr)

			return item, postErr
		}

		return next != nil && next.Title == "Repaired", postErr
	}

	err = runner.Run(t.Context(), spec)
	if err != nil {
		t.Fatalf("Run = %v", err)
	}

	refusal := "unknown Story step key: " + key
	if len(requests) != 2 || strings.Contains(requests[0], refusal) ||
		!strings.Contains(requests[1], refusal) || !strings.Contains(requests[1], "acceptance criterion 001-summary") {
		t.Fatalf("fix requests = %q, want refusal in next proposal only", requests)
	}

	assertStoryFixAccepted(t, versions, initial)
}

func assertStoryFixNotTaken(t *testing.T, versions *fs.StoryVersionManager, postErr error) {
	t.Helper()

	if !errors.Is(postErr, runner.ErrFixNotTaken) {
		t.Errorf("PostFix = %v, want fix not taken", postErr)
	}

	latest, err := versions.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}

	if latest.Title != "Original" || versions.GetCurrentVersion() != 1 {
		t.Fatalf("refused Story = %v, want original version", latest)
	}
}

func assertStoryFixAccepted(t *testing.T, versions *fs.StoryVersionManager, initial *story.Story) {
	t.Helper()

	latest, err := versions.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}

	if latest.ID != initial.ID || latest.Feature != initial.Feature {
		t.Fatalf("accepted Story = %v, want original id and Feature", latest)
	}

	if len(latest.AcceptanceCriteria) != 1 || len(latest.AcceptanceCriteria[0].Steps) != 1 {
		t.Fatalf("accepted Story = %v, want repaired criterion and step", latest)
	}

	if len(latest.AcceptanceCriteria[0].Steps[0].Then) != 2 {
		t.Fatalf("accepted Story = %v, want nested modifier", latest)
	}
}
