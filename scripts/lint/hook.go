package lint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
)

// `reason` is DISCARDED unless `decision` is "block", so a finding sent
// without one is a finding nobody reads.
const blockDecision = "block"

const hookAdvice = `LINT FAILED on %s. Fix it in that file now, before any other work:
these same gates run at commit time and reject the branch otherwise. What was
auto-fixable is already applied; what follows needs a real edit.

%s`

// Hook lints the file Claude just wrote and hands the verdict straight back.
// Everything unjudgeable — no file_path, a path outside the repository, an
// ignored file — says nothing at all.
func Hook(in io.Reader, out io.Writer) error {
	payload, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading the tool payload: %w", err)
	}

	var event struct {
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
	}

	err = json.Unmarshal(payload, &event)
	if err != nil {
		// A payload this hook cannot read is not a lint finding.
		return nil //nolint:nilerr // unreadable input is silence, not a verdict.
	}

	relative, ok := inRepository(event.ToolInput.FilePath)
	if !ok {
		return nil
	}

	var captured bytes.Buffer
	if Dispatch(&captured, []string{relative}) == nil {
		return nil
	}

	verdict, err := json.Marshal(map[string]string{
		"decision": blockDecision,
		"reason":   fmt.Sprintf(hookAdvice, relative, captured.String()),
	})
	if err != nil {
		return fmt.Errorf("encoding the verdict: %w", err)
	}

	_, _ = fmt.Fprintf(out, "%s\n", verdict)

	return nil
}

// inRepository makes the path repo-relative, and reports false for anything
// this gate has no opinion about.
func inRepository(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	root, err := filepath.Abs(".")
	if err != nil {
		return "", false
	}

	relative, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", false
	}

	ignored, err := git.IsIgnored(context.Background(), relative)
	if err != nil || ignored {
		return "", false
	}

	return relative, true
}
