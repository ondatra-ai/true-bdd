package taskhandle

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

// The tool allowlists. Deliberately verb-by-verb rather than `Bash(go *)`:
// that blanket would let a turn run ./scripts/cmd/commit or .../merge itself,
// and those two are package imports here, never subprocesses.
const (
	planTools = "Read,Glob,Grep"

	editTools = "Read,Edit,Write,Glob,Grep," +
		"Bash(git --no-pager diff *),Bash(git --no-pager log *),Bash(git status *)," +
		"Bash(git checkout -- *),Bash(git restore *)," +
		"Bash(go build *),Bash(go test *),Bash(go vet *),Bash(gofmt *)," +
		"Bash(golangci-lint *),Bash(go run ./scripts/cmd/linters *)," +
		"Bash(" + merge.Gates + ")"

	// The only allowlist here carrying Skill, and read-only besides: step 6
	// invokes code-review, which has no Go implementation to call instead.
	reviewTools = "Read,Glob,Grep,Skill,Task," +
		"Bash(git --no-pager diff *),Bash(git --no-pager log *)"
)

// allowlists is every allowlist by name, so a test can hold them to the rule
// that commit and merge are never reachable from a turn.
func allowlists() map[string]string {
	return map[string]string{
		"plan": planTools, "edit": editTools, "review": reviewTools,
	}
}

const (
	defaultPlanTimeout      = 900 * time.Second
	defaultImplementTimeout = 3600 * time.Second
	defaultReviewTimeout    = 1800 * time.Second
)

// plan is what the planning turn returns.
type plan struct {
	Plan    string   `json:"plan"`
	Files   []string `json:"files"`
	Refusal string   `json:"refusal"`
}

const planSchema = `{"type":"object","required":["plan","files","refusal"],` +
	`"properties":{"plan":{"type":"string"},` +
	`"files":{"type":"array","items":{"type":"string"}},` +
	`"refusal":{"type":"string"}}}`

// build is what every editing turn returns — implement, fix and narrow answer
// in one shape, so one caller reads all three.
type build struct {
	FilesChanged []string `json:"files_changed"`
	GatesGreen   bool     `json:"gates_green"`
	Summary      string   `json:"summary"`
}

const buildSchema = `{"type":"object",` +
	`"required":["files_changed","gates_green","summary"],` +
	`"properties":{"files_changed":{"type":"array","items":{"type":"string"}},` +
	`"gates_green":{"type":"boolean"},"summary":{"type":"string"}}}`

// Finding is one thing step 6's review said about the branch.
type Finding struct {
	Kind  string `json:"kind"`
	What  string `json:"what"`
	Where string `json:"where"`
}

// review is what the review turn returns.
type review struct {
	SpecFindings      []Finding `json:"spec_findings"`
	StandardsFindings []string  `json:"standards_findings"`
}

const reviewSchema = `{"type":"object",` +
	`"required":["spec_findings","standards_findings"],` +
	`"properties":{"spec_findings":{"type":"array","items":{"type":"object",` +
	`"required":["kind","what","where"],"properties":{` +
	`"kind":{"type":"string","enum":["missing","wrong","unasked"]},` +
	`"what":{"type":"string"},"where":{"type":"string"}}}},` +
	`"standards_findings":{"type":"array","items":{"type":"string"}}}}`

// turn runs one schema'd turn and decodes it. Every caller wants the same
// three things — the allowlist, the schema, and the answer as a struct.
func turn[T any](prompt, tools, schema, mode, role string, timeout time.Duration) (T, error) {
	var answer T

	raw, err := claudecli.RunJSON(prompt, claudecli.Options{
		AllowedTools:   tools,
		PermissionMode: mode,
		Schema:         schema,
		Role:           role,
		Timeout:        timeout,
	})
	if err != nil {
		return answer, fmt.Errorf("the %s turn failed: %w", role, err)
	}

	if len(raw) == 0 {
		return answer, fmt.Errorf("the %s turn returned %w", role, errNoStructuredOutput)
	}

	err = json.Unmarshal(raw, &answer)
	if err != nil {
		return answer, fmt.Errorf("reading the %s turn's answer: %w", role, err)
	}

	return answer, nil
}

// fill substitutes a prompt's placeholders.
func fill(prompt string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(prompt)
}
