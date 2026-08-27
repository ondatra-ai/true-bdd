package commit_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

// The branch step ran on a green tree and stopped the whole commit, because
// the prompt's `feat/` was answered as a conventional-commit `feat:`. Every
// case here is a shape git refuses outright.
func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		answer string
		want   string
	}{
		{
			name:   "a conventional-commit subject becomes a path",
			answer: "feat: clickup-defer-and-backlog-status",
			want:   "feat/clickup-defer-and-backlog-status",
		},
		{"a colon with no space", "fix:the-thing", "fix/the-thing"},
		{"spaces inside the description", "feat/add ticket template", "feat/add-ticket-template"},
		{"a name git would read as a flag", "--force", "force"},
		{"upper case", "Feat/Add-Template", "feat/add-template"},
		{"a fenced or quoted answer", `"feat/quoted"`, "feat/quoted"},
		{"a trailing slash", "feat/", "feat"},
		{"an answer with nothing usable", "!!! ???", ""},
		{"an already valid name is untouched", "feat/ticket-template", "feat/ticket-template"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := commit.SanitizeBranchName(testCase.answer); got != testCase.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", testCase.answer, got, testCase.want)
			}
		})
	}
}

// The 60-char cap must not leave an edge git rejects.
func TestSanitizeBranchNameTruncatesToAValidRef(t *testing.T) {
	t.Parallel()

	got := commit.SanitizeBranchName("feat/" + string(make([]byte, 0)) +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-tail")

	const limit = 60
	if len(got) > limit {
		t.Errorf("name is %d chars, want at most %d", len(got), limit)
	}

	if got[len(got)-1] == '-' || got[len(got)-1] == '/' {
		t.Errorf("name %q ends on an edge git rejects", got)
	}
}
