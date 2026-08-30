package github

import (
	"fmt"
	"strconv"
	"strings"
)

// perPage is the largest page the REST API serves, so --paginate makes the
// fewest round trips.
const perPage = 100

// threadQuery reads a pull request's review threads. GitHub's GraphQL field
// names are its contract, not ours; the page caps are what ReviewThreads
// refuses on rather than reporting a floor as a total.
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

// resolveMutation marks one review thread resolved.
const resolveMutation = "query=mutation($id:ID!)" +
	"{resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}"

// IssueComments decodes every comment on a pull request's conversation. No
// --jq: `api --paginate` filters PER PAGE, so a filter emits one answer a page
// rather than one for the query.
func IssueComments(target any, repo string, number int) error {
	return decode(target, "api", "--paginate", issuePath(repo, number, "comments"))
}

// Reviews decodes every review object on a pull request, paginated the same
// way and for the same reason.
func Reviews(target any, repo string, number int) error {
	return decode(target, "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=%d", repo, number, perPage))
}

// IssueComment decodes one comment by its database id.
func IssueComment(target any, repo string, commentID int) error {
	return decode(target, "api", fmt.Sprintf("repos/%s/issues/comments/%d", repo, commentID))
}

// ReplyToReviewComment answers one review comment in its own thread.
func ReplyToReviewComment(repo string, number, commentID int, body string) error {
	return silent("api", "-X", "POST",
		fmt.Sprintf("repos/%s/pulls/%d/comments/%d/replies", repo, number, commentID),
		"-f", "body="+body, "--jq", ".id")
}

// ReviewThreads decodes a pull request's review threads, which only GraphQL
// reports. repo is `owner/name`, split here because the query takes them apart.
func ReviewThreads(target any, repo string, number int) error {
	owner, name, _ := strings.Cut(repo, "/")

	return decode(target, "api", "graphql",
		"-f", "query="+threadQuery,
		"-F", "owner="+owner,
		"-F", "repo="+name,
		"-F", "pr="+strconv.Itoa(number))
}

// ResolveReviewThread marks a review thread resolved.
func ResolveReviewThread(threadID string) error {
	return silent("api", "graphql", "-f", resolveMutation,
		"-F", "id="+threadID,
		"--jq", ".data.resolveReviewThread.thread.isResolved")
}

// LastCompletedRun is the database id of the newest completed run of a
// workflow on a branch, empty when the workflow has never finished there.
func LastCompletedRun(workflow, branch string) (string, error) {
	result, err := run("run", "list", "--workflow", workflow, "--branch", branch,
		"--status", "completed", "--limit", "1",
		"--json", "databaseId", "--jq", ".[].databaseId")
	if err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", nil
	}

	return strings.TrimSpace(result.Stdout), nil
}

// RunJobs is a workflow run's jobs as the API's raw JSON, which the caller
// folds into its own shape.
func RunJobs(repo, runID string) ([]byte, error) {
	out, err := output("api", "repos/"+repo+"/actions/runs/"+runID+"/jobs")
	if err != nil {
		return nil, err
	}

	return []byte(out), nil
}

// issuePath is the conversation-comments endpoint, which lives under `issues`
// because GitHub models a pull request as one.
func issuePath(repo string, number int, leaf string) string {
	return fmt.Sprintf("repos/%s/issues/%d/%s?per_page=%d", repo, number, leaf, perPage)
}
