package merge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

const threadQuery = `
query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      reviewThreads(first:100){
        pageInfo{ hasNextPage }
        nodes{
          id isResolved isOutdated path line
          comments(first:20){ pageInfo{ hasNextPage } nodes{ databaseId author{login} body } }
        }
      }
    }
  }
}
`

// GitHub's GraphQL field names are its contract, not ours.
//
//nolint:tagliatelle // camelCase is what the API returns.
type (
	pageInfo struct {
		HasNextPage bool `json:"hasNextPage"`
	}

	threadAuthor struct {
		Login string `json:"login"`
	}

	threadComment struct {
		DatabaseID int           `json:"databaseId"`
		Author     *threadAuthor `json:"author"`
		Body       string        `json:"body"`
	}

	reviewThread struct {
		ID         string `json:"id"`
		IsResolved bool   `json:"isResolved"`
		IsOutdated bool   `json:"isOutdated"`
		Path       string `json:"path"`
		Line       *int   `json:"line"`
		Comments   struct {
			PageInfo pageInfo        `json:"pageInfo"`
			Nodes    []threadComment `json:"nodes"`
		} `json:"comments"`
	}

	threadsResponse struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						PageInfo pageInfo       `json:"pageInfo"`
						Nodes    []reviewThread `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
)

// authorOf names a comment's author.
//
// GitHub returns author: null for a deleted account, and reaching through it
// would abort the one routine whose job is to miss nothing.
func authorOf(comment threadComment) string {
	if comment.Author == nil || comment.Author.Login == "" {
		return "(deleted account)"
	}

	return comment.Author.Login
}

func (r *Run) fetchThreads() []reviewThread {
	owner, name, _ := strings.Cut(r.repo, "/")

	var response threadsResponse

	r.ghJSON(&response, "api", "graphql",
		"-f", "query="+threadQuery,
		"-F", "owner="+owner,
		"-F", "repo="+name,
		"-F", "pr="+strconv.Itoa(r.pr))

	threads := response.Data.Repository.PullRequest.ReviewThreads

	// A truncated page turns every count below into a floor while still
	// reading as a total. "Could not answer" is not "nothing to report".
	if threads.PageInfo.HasNextPage {
		r.dief("reviewThreads is truncated at 100 — paginate the query before trusting any count")
	}

	for _, thread := range threads.Nodes {
		if thread.Comments.PageInfo.HasNextPage {
			r.dief("thread %s has more than 20 comments — paginate before trusting it", thread.ID)
		}
	}

	return threads.Nodes
}

// readComments is every finding on the PR: unresolved threads plus body-only
// findings.
func (r *Run) readComments() []clickup.Finding {
	threads := r.fetchThreads()
	reviews := r.reviews()

	var findings []clickup.Finding

	for _, thread := range threads {
		if thread.IsResolved || len(thread.Comments.Nodes) == 0 {
			continue
		}

		findings = append(findings, threadFinding(thread))
	}

	var bodyOnly []bodyFinding

	for _, review := range reviews {
		if isBot(review.User.Login) {
			bodyOnly = append(bodyOnly, extractBodyFindings(review)...)
		}
	}

	for index, item := range bodyOnly {
		findings = append(findings, bodyOnlyFinding(index, item))
	}

	unique := dedupe(findings)
	r.reconcile(threads, bodyOnly, reviews, len(unique))

	return unique
}

func threadFinding(thread reviewThread) clickup.Finding {
	head := thread.Comments.Nodes[0]
	body := clean(head.Body)

	line := "?"
	if thread.Line != nil && *thread.Line != 0 {
		line = strconv.Itoa(*thread.Line)
	}

	path := thread.Path
	if path == "" {
		path = "?"
	}

	databaseID := head.DatabaseID
	threadID := thread.ID

	return clickup.Finding{
		ID:        "thread:" + thread.ID,
		Source:    "thread",
		Severity:  severityOf(body),
		File:      path,
		Line:      line,
		Title:     titleOf(body),
		Body:      body,
		CommentID: &databaseID,
		ThreadID:  &threadID,
		Author:    authorOf(head),
	}
}

func bodyOnlyFinding(index int, item bodyFinding) clickup.Finding {
	line := item.lines
	if line == "?" {
		line = lineFromBody(item.text)
	}

	return clickup.Finding{
		// Indexed: several body-only findings share a file and often have no
		// parsed line, and a collision silently gives every colliding finding
		// the first one's score.
		ID:       fmt.Sprintf("body:%d:%s", index, item.path),
		Source:   "body-only",
		Severity: severityOf(item.text),
		File:     item.path,
		Line:     line,
		Title:    titleOf(item.text),
		Body:     item.text,
		Author:   "coderabbitai[bot]",
	}
}

// dedupe drops repeats: the same defect reaches us as a thread and again in
// the body often enough to matter.
func dedupe(findings []clickup.Finding) []clickup.Finding {
	const keyTitleWidth = 80

	seen := map[string]bool{}
	unique := make([]clickup.Finding, 0, len(findings))

	for _, finding := range findings {
		key := finding.File + "\x00" + finding.Line + "\x00" + truncate(finding.Title, keyTitleWidth)
		if seen[key] {
			continue
		}

		seen[key] = true

		unique = append(unique, finding)
	}

	return unique
}

// claimedCounts is what the review bodies say about themselves, summed.
func claimedCounts(reviews []ghReview) map[string]int {
	counts := map[string]int{"actionable": 0}
	for _, section := range bodyOnlySections {
		counts[section] = 0
	}

	for _, review := range reviews {
		if !isBot(review.User.Login) {
			continue
		}

		for _, match := range actionableRE.FindAllStringSubmatch(review.Body, -1) {
			counts["actionable"] += atoi(match[1])
		}

		for _, match := range sectionRE.FindAllStringSubmatch(review.Body, -1) {
			counts[match[1]] += atoi(match[2])
		}
	}

	return counts
}

// reconcile compares what we extracted against what the reviews claim.
//
// A WARNING, never a stop. CodeRabbit over-counts — review 4960672409
// announced "Actionable comments posted: 6" and posted 5 — so treating the gap
// as fatal made the guard unsatisfiable and stalled PR #76's merge. A gap can
// mean a missed finding OR a bot over-count; both get printed.
func (r *Run) reconcile(threads []reviewThread, bodyOnly []bodyFinding, reviews []ghReview, uniqueCount int) {
	claimed := claimedCounts(reviews)

	botThreads := 0

	for _, thread := range threads {
		if len(thread.Comments.Nodes) > 0 && isBot(authorOf(thread.Comments.Nodes[0])) {
			botThreads++
		}
	}

	claimedBody := 0
	for _, section := range bodyOnlySections {
		claimedBody += claimed[section]
	}

	var gaps []string

	if botThreads != claimed["actionable"] {
		gaps = append(gaps, fmt.Sprintf("actionable: %d thread(s) found, %d claimed",
			botThreads, claimed["actionable"]))
	}

	if len(bodyOnly) != claimedBody {
		gaps = append(gaps, fmt.Sprintf("body-only: %d extracted, %d claimed",
			len(bodyOnly), claimedBody))
	}

	r.logf("%d thread(s), %d body-only, %d after dedupe", len(threads), len(bodyOnly), uniqueCount)

	if len(gaps) > 0 {
		r.logf("! reconciliation gap — %s", strings.Join(gaps, "; "))
		r.logf("  (CodeRabbit over-counts; continuing with what is actually there)")

		return
	}

	r.logf("reconciled against the reviews' own counts")
}

func atoi(text string) int {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}

	return value
}

// truncate counts runes, as the Python slices it ports did.
func truncate(text string, limit int) string {
	return textutil.Truncate(text, limit)
}
