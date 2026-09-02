package clickup_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// listTasks returns a `not relevant` task even with includeClosed false
// (probed 2026-09-02), so only this sentence keeps a retired ticket from
// reading as open and suppressing a legitimate filing via dropAlreadyOpen.
func TestListPromptExcludesEverySettledStatus(t *testing.T) {
	t.Parallel()

	prompt := clickup.ListPromptForTest("merge-improvements")

	for _, status := range []string{notRelevant, "done", "failed"} {
		if !strings.Contains(prompt, `"`+status+`"`) {
			t.Errorf("the listing prompt does not exclude %q:\n%s", status, prompt)
		}
	}
}

func TestListPromptNamesTheTag(t *testing.T) {
	t.Parallel()

	if got := clickup.ListPromptForTest("fix-now"); !strings.Contains(got, "`fix-now`") {
		t.Fatalf("the listing prompt does not name the tag it was given:\n%s", got)
	}
}
