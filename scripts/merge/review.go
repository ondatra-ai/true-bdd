package merge

import (
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What CodeRabbit answers a review request with — one of these three.
// The predecessor didn't check: PR #76 requested four reviews in 25 min,
// was refused each time, and waited 900s per attempt for nothing.
const (
	ackRateLimited = "Review rate limited"
	// The third answer also arrives under "⚠️ Action not completed":
	//     ⚠️ Action not completed
	//     No files to review.
	// path_filters scopes review to tests/: PR #81's tests/-only diff got this
	// and used to fall through to the ackBudget timeout as a false "bot may be down" report.
	ackNothingToReview = "No files to review"
)

//nolint:gochecknoglobals // the accepted markers and the wait pattern are constants in all but syntax.
var (
	ackAccepted = []string{"Full review triggered", "Action performed"}
	// "Your next included review will be available in 18 minutes." — the bot
	// says how long, so there is no reason to guess.
	ackWaitRE = regexp.MustCompile(`available in (\d+)\s*minute`)
)

// verdict is what the bot said about a review request.
type verdict int

const (
	verdictSilent verdict = iota
	verdictAccepted
	verdictRateLimited
	verdictNothingToReview
)

// String names the verdict in the log, where an iota is unreadable.
func (v verdict) String() string {
	switch v {
	case verdictSilent:
		return "silent"
	case verdictAccepted:
		return "accepted"
	case verdictRateLimited:
		return "rate limited"
	case verdictNothingToReview:
		return "nothing in review scope"
	default:
		return "unknown"
	}
}

type ghUser struct {
	Login string `json:"login"`
}

type ghComment struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
	User ghUser `json:"user"`
}

type ghReview struct {
	ID       int    `json:"id"`
	Body     string `json:"body"`
	CommitID string `json:"commit_id"`
	User     ghUser `json:"user"`
}

// requestReview asks round 1 for a full review and later rounds for an
// incremental one: those add only fix commits, so a full re-review re-reports
// what round 1 already triaged. HEAD must be pushed, or the review misses it.
func (r *Run) requestReview(round int) {
	defer report.Open("request review", "fixes", round <= lastFixRound)()

	r.checkPushed()

	command, kind := "@coderabbitai review", "an incremental review"
	if round == 1 {
		command, kind = "@coderabbitai full review", "a full review"
	}

	for attempt := 1; ; attempt++ {
		baselineReview, baselineComment := r.newestReviewID(), r.newestCommentID()

		r.logf("requesting %s of %s (attempt %d)", kind, short(r.headSHA()), attempt)
		r.check("requesting the review", github.Comment(r.pr, command))

		answer, ackID := r.awaitAcknowledgement(baselineComment)

		switch answer {
		case verdictSilent:
			r.dief("CodeRabbit did not answer the review request within %s. "+
				"Check https://github.com/%s/pull/%d — the bot may be down.", ackBudget, r.repo, r.pr)
		case verdictNothingToReview:
			// Zero files reviewed is a completed review, not a refusal — retrying
			// gets the same answer forever. Mark headSHA reviewed now so `merge`
			// doesn't reject the empty-bodied APPROVED CodeRabbit already posted.
			r.reviewedThisRun[r.headSHA()] = true

			return
		case verdictAccepted:
			if r.awaitReview(baselineReview, ackID) {
				return
			}
		case verdictRateLimited:
		}

		r.sleepOffRateLimit(r.commentBody(ackID))
	}
}

// awaitAcknowledgement waits for the bot's answer to a request. Check
// ackNothingToReview before ackRateLimited: both share the "Action not
// completed" heading; only the wording tells a spent quota from an empty diff.
func (r *Run) awaitAcknowledgement(baselineComment int) (verdict, int) {
	// Measured, not counted: both loops here sleep BEFORE their first read, so
	// a tick count is a 30s floor that overstates every wait it reports.
	started := time.Now()

	answer, id := r.pollAcknowledgement(baselineComment)

	report.Leaf("await acknowledgement", started, "answer", answer.String())

	return answer, id
}

func (r *Run) pollAcknowledgement(baselineComment int) (verdict, int) {
	for waited := time.Duration(0); waited < ackBudget; waited += poll {
		time.Sleep(poll)

		for _, comment := range r.botCommentsAfter(baselineComment) {
			switch {
			case strings.Contains(comment.Body, ackNothingToReview):
				return verdictNothingToReview, comment.ID
			case strings.Contains(comment.Body, ackRateLimited):
				return verdictRateLimited, comment.ID
			case containsAny(comment.Body, ackAccepted):
				return verdictAccepted, comment.ID
			}
		}
	}

	return verdictSilent, 0
}

// awaitReview waits for a review OBJECT; false means the ack was later
// edited to a rate limit. Re-read every poll: PR #77 comment 5330633865 was
// posted "triggered" then edited 32s later to "quota spent" — reading once misses that.
func (r *Run) awaitReview(baselineReview, ackID int) bool {
	started := time.Now()

	posted := r.pollReview(baselineReview, ackID)

	report.Leaf("await review", started, "posted", posted)

	return posted
}

func (r *Run) pollReview(baselineReview, ackID int) bool {
	for waited := time.Duration(0); waited < reviewBudget; waited += poll {
		time.Sleep(poll)

		if r.newestReviewID() > baselineReview {
			r.recordReviewsAfter(baselineReview)

			return true
		}

		if strings.Contains(r.commentBody(ackID), ackRateLimited) {
			r.logf("the acknowledgement was edited to a rate limit")

			return false
		}
	}

	r.dief("no review object within %s of an accepted request. The bot acknowledged "+
		"and did not deliver — read the PR before re-running.", reviewBudget)

	return false
}

// sleepOffRateLimit waits out the quota, for as long as the bot says it needs.
func (r *Run) sleepOffRateLimit(body string) {
	wait, named := rateLimitWait(body)

	suffix := "."
	if named {
		suffix = " (the bot named the wait)."
	}

	r.logf("rate limited — the review quota is spent. Sleeping %d min%s",
		int(wait.Minutes()), suffix)
	time.Sleep(wait)
}

func rateLimitWait(body string) (time.Duration, bool) {
	match := ackWaitRE.FindStringSubmatch(body)
	if match == nil {
		return rateLimitSleep, false
	}

	minutes, err := strconv.Atoi(match[1])
	if err != nil {
		return rateLimitSleep, false
	}

	// A minute of slack: the bot's estimate is when the quota refills, not
	// when a request made at that instant succeeds.
	return time.Duration(minutes)*time.Minute + time.Minute, true
}

// recordReviewsAfter notes which commits got reviews newer than sinceID.
// Call it the moment a review lands — that's the only point where "reviewed,
// found nothing" is still distinguishable from a later rubber stamp.
func (r *Run) recordReviewsAfter(sinceID int) {
	for _, review := range r.reviews() {
		if review.ID > sinceID && isBot(review.User.Login) {
			r.reviewedThisRun[review.CommitID] = true
		}
	}
}

// ------------------------------------------------------------ GitHub reads

func (r *Run) comments() []ghComment {
	var payload []ghComment

	r.check("reading the conversation", github.IssueComments(&payload, r.repo, r.pr))

	return payload
}

func (r *Run) reviews() []ghReview {
	var payload []ghReview

	r.check("reading the reviews", github.Reviews(&payload, r.repo, r.pr))

	return payload
}

// newestCommentID is the high-water mark a review request is measured against.
func (r *Run) newestCommentID() int {
	newest := 0
	for _, comment := range r.comments() {
		newest = max(newest, comment.ID)
	}

	return newest
}

// newestReviewID is the same for review objects.
//
// No --jq: gh api --paginate filters per page, so a `max` filter emits one per page.
func (r *Run) newestReviewID() int {
	newest := 0
	for _, review := range r.reviews() {
		newest = max(newest, review.ID)
	}

	return newest
}

func (r *Run) botCommentsAfter(sinceID int) []ghComment {
	var replies []ghComment

	for _, comment := range r.comments() {
		if comment.ID > sinceID && isBot(comment.User.Login) {
			replies = append(replies, comment)
		}
	}

	return replies
}

func (r *Run) commentBody(commentID int) string {
	if commentID == 0 {
		return ""
	}

	var payload ghComment

	r.check("reading a comment", github.IssueComment(&payload, r.repo, commentID))

	return payload.Body
}

func containsAny(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}

	return false
}

func short(sha string) string {
	const width = 7

	if len(sha) <= width {
		return sha
	}

	return sha[:width]
}
