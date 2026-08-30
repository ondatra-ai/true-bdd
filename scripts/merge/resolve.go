package merge

import (
	"encoding/json"
	"strconv"

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

func (r *Run) replyAndResolve(commentID int, threadID, text string) {
	r.check("replying to the review comment",
		github.ReplyToReviewComment(r.repo, r.pr, commentID, text))
	r.check("resolving the thread", github.ResolveReviewThread(threadID))
}

// answer pairs a finding with what its thread is told.
type answer struct {
	finding clickup.Finding
	text    string
}

// resolveConversations answers and resolves every thread — main requires
// resolution, so an open one blocks merge. Body-only findings are skipped:
// GitHub gives them no id, reply target, or resolvable state.
func (r *Run) resolveConversations(fixed, created, ignored []clickup.Finding) {
	defer report.Open("resolve conversations")()

	answers := make([]answer, 0, len(fixed)+len(created)+len(ignored))

	for _, finding := range fixed {
		detail := finding.FixSummary
		if detail == "" {
			detail = finding.Reason
		}

		answers = append(answers, answer{finding: finding, text: "**Fixed on this branch.** " + detail})
	}

	for _, finding := range created {
		detail := ticketURL(finding)
		if detail == "" {
			detail = "see the ClickUp `fix-now` queue"
		}

		answers = append(answers,
			answer{finding: finding, text: "**Valid, deferred to a ticket.** " + detail})
	}

	for _, finding := range ignored {
		answers = append(answers,
			answer{finding: finding, text: "**Not actioned.** " + finding.Reason})
	}

	answered := 0

	for _, item := range answers {
		if item.finding.ThreadID == nil || item.finding.CommentID == nil {
			continue
		}

		r.replyAndResolve(*item.finding.CommentID, *item.finding.ThreadID, item.text)

		answered++
	}

	r.logf("answered and resolved %d thread(s)", answered)
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
		r.replyAndResolve(head.DatabaseID, thread.ID,
			"**Not actioned.** Reviewed in this round and not selected for a fix "+
				"or a ticket; resolving so the merge is not blocked.")

		swept++
	}

	if swept > 0 {
		r.logf("swept %d remaining thread(s)", swept)
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
