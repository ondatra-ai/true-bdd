package codexrun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gh"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func (r Runner) saveIssues(ctx context.Context, record Record) error {
	if record.SessionID != "" {
		return nil
	}

	for _, number := range record.Issues {
		path := r.file(record.Name, "issues/"+number+".md")

		_, err := disk.Read(path)
		if err == nil {
			continue
		}

		data, err := gh.IssueJSON(ctx, r.Root, number, r.watch())
		if err != nil {
			return fmt.Errorf("saving issue #%s: %w", number, err)
		}

		body, err := issueMarkdown(data)
		if err != nil {
			return err
		}

		err = disk.Write(path, []byte(body), disk.Private)
		if err != nil {
			return err
		}
	}

	return nil
}

func issueMarkdown(data []byte) (string, error) {
	var issue struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			CreatedAt string `json:"createdAt"` //nolint:tagliatelle // GitHub's API wire spelling.
			Body      string `json:"body"`
		} `json:"comments"`
	}

	err := json.Unmarshal(data, &issue)
	if err != nil {
		return "", fmt.Errorf("decoding issue: %w", err)
	}

	var text strings.Builder
	fmt.Fprintf(&text, "# #%d %s\n\nState: %s\n\n%s\n", issue.Number, issue.Title, issue.State, issue.Body)

	for _, label := range issue.Labels {
		fmt.Fprintf(&text, "\nLabel: %s\n", label.Name)
	}

	if len(issue.Comments) > 0 {
		text.WriteString("\n## Comments\n")
	}

	for _, comment := range issue.Comments {
		fmt.Fprintf(&text, "\n### %s, %s\n\n%s\n", comment.Author.Login, comment.CreatedAt, comment.Body)
	}

	return text.String(), nil
}
