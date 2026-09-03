package triage_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/triage"
)

// The floor is restated here so the boundary case fails if it ever moves.
const (
	floor = 6
	// reason and story stand in for any non-empty ones: validate only checks
	// that they are not blank.
	reason = "why"
	story  = "run.go:112 calls it; a `--fix` run then files nothing"
)

// why is the first heading; shape is all four, named here rather than taken
// from scripts/clickup, which triage must not import.
const why = "Why"

//nolint:gochecknoglobals // a table's fixture, not state.
var shape = []string{why, "What to change", "Verification", "Context"}

const conforming = "### Why\n\nit still bites.\n\n### What to change\n\n`run.go:112`\n\n" +
	"### Verification\n\n```bash\ngo test ./...\n```\n\n### Context\n\nfiled from #89."

func TestValidateRejectsWhatTheSchemaCannot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		verdict  triage.Verdict
		refresh  bool
		headings []string
		wantErr  bool
	}{
		{
			"a scored finding",
			triage.Verdict{Score: 9, Reason: "run.go:112 still calls it", Story: story},
			false, nil, false,
		},
		{"zero is below the band", triage.Verdict{Score: 0, Reason: reason, Story: story}, false, nil, true},
		{"eleven is above it", triage.Verdict{Score: 11, Reason: reason, Story: story}, false, nil, true},
		{"one is in the band", triage.Verdict{Score: 1, Reason: "the file is gone"}, false, nil, false},
		{"ten is in the band", triage.Verdict{Score: 10, Reason: reason, Story: story}, false, nil, false},
		{"a score with no reason", triage.Verdict{Score: 7, Story: story}, false, nil, true},
		{"a reason of only spaces", triage.Verdict{Score: 7, Reason: "   ", Story: story}, false, nil, true},
		{
			"at the floor under refresh, with no description",
			triage.Verdict{Score: floor, Reason: reason}, true, nil, true,
		},
		{
			"at the floor under refresh, with one",
			triage.Verdict{Score: floor, Reason: reason, Description: "### Why\n\n…"},
			true, nil, false,
		},
		{
			"below the floor under refresh needs none",
			triage.Verdict{Score: floor - 1, Reason: reason}, true, nil, false,
		},
		{
			"at the floor WITHOUT refresh needs no description",
			triage.Verdict{Score: floor, Reason: reason, Story: story}, false, nil, false,
		},
		{
			"at the floor WITHOUT refresh, with no story",
			triage.Verdict{Score: floor, Reason: reason}, false, nil, true,
		},
		{
			"a story of only spaces",
			triage.Verdict{Score: floor, Reason: reason, Story: "  \n "}, false, nil, true,
		},
		{
			"below the floor needs no story",
			triage.Verdict{Score: floor - 1, Reason: reason}, false, nil, false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := triage.ValidateForTest(test.verdict, test.refresh, test.headings)
			if (err != nil) != test.wantErr {
				t.Errorf("validate(%+v, refresh=%v) = %v, wantErr %v",
					test.verdict, test.refresh, err, test.wantErr)
			}
		})
	}
}

// A refreshed body that lost a heading is rejected, not written: every reader
// afterwards finds the section by its `### ` line, and ClickUp hands back only
// what was stored. Rejecting here spends Score's retry instead.
func TestValidateHoldsARefreshedBodyToItsHeadings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		body     string
		headings []string
		wantErr  bool
	}{
		{"missing three of the four", "### Why\n\n…", shape, true},
		{"carrying every one", conforming, shape, false},
		{"a heading named inside a paragraph is prose", "see ### Why above", []string{why}, true},
		{"named none, checked for none", "no headings at all", nil, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			verdict := triage.Verdict{Score: floor, Reason: reason, Description: test.body}

			err := triage.ValidateForTest(verdict, true, test.headings)
			if (err != nil) != test.wantErr {
				t.Errorf("validate(%q, %q) = %v, wantErr %v",
					test.body, test.headings, err, test.wantErr)
			}
		})
	}
}

// Below the floor the ticket is retired, so its body is never reshaped.
func TestValidateLeavesARetiredBodyAlone(t *testing.T) {
	t.Parallel()

	verdict := triage.Verdict{Score: floor - 1, Reason: reason}

	err := triage.ValidateForTest(verdict, true, shape)
	if err != nil {
		t.Errorf("a ticket about to be retired was held to the headings: %v", err)
	}
}

// The floor is the one number three callers dispose on. A change to it is a
// change to what merge tickets, what the sweep retires and what defer refuses.
func TestFloorIsSix(t *testing.T) {
	t.Parallel()

	if triage.Floor != floor {
		t.Errorf("triage.Floor = %d, want %d", triage.Floor, floor)
	}
}

// The schema is handed to --json-schema, which refuses to run on one that does
// not parse — and refuses at the first turn, long after the build went green.
func TestSchemaParsesAndBoundsTheBand(t *testing.T) {
	t.Parallel()

	var schema struct {
		Required   []string `json:"required"`
		Properties struct {
			Score struct {
				Type    string `json:"type"`
				Minimum int    `json:"minimum"`
				Maximum int    `json:"maximum"`
			} `json:"score"`
		} `json:"properties"`
	}

	err := json.Unmarshal([]byte(triage.SchemaForTest()), &schema)
	if err != nil {
		t.Fatalf("the verdict schema does not parse: %v", err)
	}

	if got := strings.Join(schema.Required, ","); got != "score,reason,description,story" {
		t.Errorf("required = %q, want all four fields", got)
	}

	if schema.Properties.Score.Minimum != 1 || schema.Properties.Score.Maximum != 10 {
		t.Errorf("score bounded %d-%d, want 1-10",
			schema.Properties.Score.Minimum, schema.Properties.Score.Maximum)
	}
}
