package merge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What CodeRabbit answers a review request with. It always answers one of
// these three, and nothing in the predecessor read the answer, so PR #76
// requested four reviews in 25 minutes, was refused, and waited 900s each
// time for a review that was never going to arrive.
const (
	ackRateLimited = "Review rate limited"
	// The third answer, and it also arrives under "⚠️ Action not completed":
	//
	//     ⚠️ Action not completed
	//     No files to review.
	//
	// `path_filters` scopes review to tests/, so a PR whose diff touches
	// nothing there gives the bot nothing to do. It says so in seconds and
	// posts an APPROVED review with an empty body. Measured on PR #81, whose
	// own changes against main are rules and a plugin. Not matching this used
	// to fall through to the ackBudget timeout and report "the bot may be
	// down" about a bot that had answered in ten seconds.
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

// requestReview asks for a full review, waiting out the rate limit for as
// long as it takes.
//
// Being pushed already is a precondition of asking: a review of a commit
// origin does not have is a review of the wrong thing.
func (r *Run) requestReview(round int) {
	headline := fmt.Sprintf("round %d of %d", round, lastRound)
	if round > lastFixRound {
		headline += " — triage only, nothing is fixed"
	}

	r.banner(headline)
	r.checkPushed()

	for attempt := 1; ; attempt++ {
		baselineReview, baselineComment := r.newestReviewID(), r.newestCommentID()

		r.logf("requesting a full review of %s (attempt %d)", short(r.headSHA()), attempt)
		r.sh([]string{"gh", "pr", "comment", strconv.Itoa(r.pr), "--body", "@coderabbitai full review"},
			options{check: true})

		answer, ackID := r.awaitAcknowledgement(baselineComment)

		switch answer {
		case verdictSilent:
			r.dief("CodeRabbit did not answer the review request within %s. "+
				"Check https://github.com/%s/pull/%d — the bot may be down.", ackBudget, r.repo, r.pr)
		case verdictNothingToReview:
			// The bot evaluated the PR and its filters left nothing. That is a
			// completed review of zero files, not a refusal: asking again would
			// get the same answer forever. HEAD counts as reviewed so `merge`
			// does not then reject the empty-bodied APPROVED it just posted.
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

// awaitAcknowledgement waits for the bot's answer to a request.
//
// Checks ackNothingToReview before ackRateLimited: both arrive under the same
// "Action not completed" heading, and only the wording separates a spent
// quota from an empty diff.
func (r *Run) awaitAcknowledgement(baselineComment int) (verdict, int) {
	for waited := time.Duration(0); waited < ackBudget; {
		time.Sleep(poll)

		waited += poll

		for _, comment := range r.botCommentsAfter(baselineComment) {
			switch {
			case strings.Contains(comment.Body, ackNothingToReview):
				r.logf("nothing in review scope after %s", waited)

				return verdictNothingToReview, comment.ID
			case strings.Contains(comment.Body, ackRateLimited):
				return verdictRateLimited, comment.ID
			case containsAny(comment.Body, ackAccepted):
				r.logf("review accepted after %s", waited)

				return verdictAccepted, comment.ID
			}
		}
	}

	return verdictSilent, 0
}

// awaitReview waits for a review OBJECT, not an acknowledgement. False means
// the acknowledgement turned out to be a rate limit after all.
//
// The acknowledgement is re-read on every poll because **CodeRabbit edits it
// in place**: on PR #77 comment 5330633865 was posted at 15:48:15 saying the
// review was triggered and rewritten at 15:48:47 to say the quota was spent.
// Reading it once caught the optimistic version and then waited the full 900s
// for a review that was never coming.
func (r *Run) awaitReview(baselineReview, ackID int) bool {
	for waited := time.Duration(0); waited < reviewBudget; {
		time.Sleep(poll)

		waited += poll

		if r.newestReviewID() > baselineReview {
			r.logf("review posted after %s", waited)
			r.recordReviewsAfter(baselineReview)

			return true
		}

		if strings.Contains(r.commentBody(ackID), ackRateLimited) {
			r.logf("the acknowledgement was edited to a rate limit after %s", waited)

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

// recordReviewsAfter notes which commits the reviews newer than sinceID were
// posted against.
//
// Called the moment a requested review lands, which is the only point at which
// "reviewed and found nothing" is still distinguishable from "rubber stamped".
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

	r.ghJSON(&payload, "api", "--paginate",
		fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", r.repo, r.pr))

	return payload
}

func (r *Run) reviews() []ghReview {
	var payload []ghReview

	r.ghJSON(&payload, "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=100", r.repo, r.pr))

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
// Deliberately reduced here rather than with `--jq`: `gh api --paginate`
// applies the filter to EACH page and concatenates, so a filter ending in
// `max` emits one number per page.
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

	r.ghJSON(&payload, "api", fmt.Sprintf("repos/%s/issues/comments/%d", r.repo, commentID))

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
