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
	// reason stands in for any non-empty one: validate only checks that.
	reason = "why"
)

func TestValidateRejectsWhatTheSchemaCannot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		verdict triage.Verdict
		refresh bool
		wantErr bool
	}{
		{"a scored finding", triage.Verdict{Score: 9, Reason: "run.go:112 still calls it"}, false, false},
		{"zero is below the band", triage.Verdict{Score: 0, Reason: reason}, false, true},
		{"eleven is above it", triage.Verdict{Score: 11, Reason: reason}, false, true},
		{"one is in the band", triage.Verdict{Score: 1, Reason: "the file is gone"}, false, false},
		{"ten is in the band", triage.Verdict{Score: 10, Reason: reason}, false, false},
		{"a score with no reason", triage.Verdict{Score: 7}, false, true},
		{"a reason of only spaces", triage.Verdict{Score: 7, Reason: "   "}, false, true},
		{
			"at the floor under refresh, with no description",
			triage.Verdict{Score: floor, Reason: reason}, true, true,
		},
		{
			"at the floor under refresh, with one",
			triage.Verdict{Score: floor, Reason: reason, Description: "### Why\n\n…"}, true, false,
		},
		{
			"below the floor under refresh needs none",
			triage.Verdict{Score: floor - 1, Reason: reason}, true, false,
		},
		{
			"at the floor WITHOUT refresh needs none",
			triage.Verdict{Score: floor, Reason: reason}, false, false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := triage.ValidateForTest(test.verdict, test.refresh)
			if (err != nil) != test.wantErr {
				t.Errorf("validate(%+v, refresh=%v) = %v, wantErr %v",
					test.verdict, test.refresh, err, test.wantErr)
			}
		})
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

	if got := strings.Join(schema.Required, ","); got != "score,reason,description" {
		t.Errorf("required = %q, want all three fields", got)
	}

	if schema.Properties.Score.Minimum != 1 || schema.Properties.Score.Maximum != 10 {
		t.Errorf("score bounded %d-%d, want 1-10",
			schema.Properties.Score.Minimum, schema.Properties.Score.Maximum)
	}
}
