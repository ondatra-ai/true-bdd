package merge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// ticketURL is the ClickUp task a finding became, from the filing record.
func ticketURL(finding clickup.Finding) string {
	const matchWidth = 60

	raw, err := disk.Read(filedRecord)
	if err != nil {
		return ""
	}

	var rows []clickup.Ticket
	if json.Unmarshal(raw, &rows) != nil {
		return ""
	}

	for _, row := range rows {
		if row.ID == "" || truncate(row.Title, matchWidth) != truncate(finding.Title, matchWidth) {
			continue
		}

		if row.URL != "" {
			return row.URL
		}

		return "task " + row.ID
	}

	return ""
}

func (r *Run) replyAndResolve(commentID int, threadID, text string) error {
	err := github.ReplyToReviewComment(r.repo, r.pr, commentID, text)
	if err != nil {
		return fmt.Errorf("replying to comment %d: %w", commentID, err)
	}

	err = github.ResolveReviewThread(threadID)
	if err != nil {
		return fmt.Errorf("resolving thread %s: %w", threadID, err)
	}

	return nil
}

// answerAll answers every answerable thread and reports what failed only once
// every one has been tried. Stopping at the first failure left the rest open
// with their tickets already filed, which is what the next run re-files.
func (r *Run) answerAll(answers []answer) int {
	answered := 0

	var failed []string

	for _, item := range answers {
		if item.finding.ThreadID == nil || item.finding.CommentID == nil {
			continue
		}

		err := r.replyAndResolve(*item.finding.CommentID, *item.finding.ThreadID, item.text)
		if err != nil {
			failed = append(failed, err.Error())

			continue
		}

		answered++
	}

	if len(failed) > 0 {
		r.dief("%d of %d thread(s) were not answered:\n  %s\n"+
			"  An unanswered thread blocks the merge and is re-filed by the next run.",
			len(failed), len(answers), strings.Join(failed, "\n  "))
	}

	return answered
}

// answerCreated tells each filed thread where its ticket is, in the step that
// filed it. Three tickets filed 2026-09-01 20:53 kept open threads when fix
// died at 20:56, and the 22:10 run filed them again (PR #114).
func (r *Run) answerCreated(created []clickup.Finding) {
	answers := make([]answer, 0, len(created))

	for _, finding := range created {
		detail := ticketURL(finding)
		if detail == "" {
			detail = "see the ClickUp `fix-now` queue"
		}

		answers = append(answers,
			answer{finding: finding, text: "**Valid, deferred to a ticket.** " + detail})
	}

	r.logf("answered and resolved %d filed thread(s)", r.answerAll(answers))
}

// answer pairs a finding with what its thread is told.
type answer struct {
	finding clickup.Finding
	text    string
}

// resolveConversations answers what the filing step did not — main requires
// resolution, so an open thread blocks merge. Body-only findings are skipped:
// GitHub gives them no id, reply target, or resolvable state at all.
func (r *Run) resolveConversations(fixed, ignored []clickup.Finding) {
	defer report.Open("resolve conversations")()

	answers := make([]answer, 0, len(fixed)+len(ignored))

	for _, finding := range fixed {
		detail := finding.FixSummary
		if detail == "" {
			detail = finding.Reason
		}

		answers = append(answers, answer{finding: finding, text: "**Fixed on this branch.** " + detail})
	}

	for _, finding := range ignored {
		answers = append(answers,
			answer{finding: finding, text: "**Not actioned.** " + finding.Reason})
	}

	r.logf("answered and resolved %d thread(s)", r.answerAll(answers))
	r.sweepStragglers()
}

// sweepStragglers resolves whatever is still open, with a reason. A thread
// opened by a human, or one whose finding deduped away, blocks the merge
// just as hard — leaving it to the end is how PR #76 got there.
func (r *Run) sweepStragglers() {
	swept := 0

	for _, thread := range r.fetchThreads() {
		if thread.IsResolved || len(thread.Comments.Nodes) == 0 {
			continue
		}

		head := thread.Comments.Nodes[0]
		r.check("sweeping a remaining thread", r.replyAndResolve(head.DatabaseID, thread.ID,
			"**Not actioned.** Reviewed in this round and not selected for a fix "+
				"or a ticket; resolving so the merge is not blocked."))

		swept++
	}

	if swept > 0 {
		r.logf("swept %d remaining thread(s)", swept)
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
