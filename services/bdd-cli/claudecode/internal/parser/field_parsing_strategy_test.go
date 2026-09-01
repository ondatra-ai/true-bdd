package parser_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode/internal/parser"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode/internal/shared"
)

// fieldResult is the stream field these strategies key off.
const fieldResult = "result"

const (
	keySubtype       = "subtype"
	keyDurationAPIMs = "duration_api_ms"
	keyIsError       = "is_error"
	keyNumTurns      = "num_turns"
	keySessionID     = "session_id"
	testSessionID    = "session123"
)

func TestRequiredFieldsStrategy(t *testing.T) {
	strategy := &parser.RequiredFieldsStrategy{}

	tests := []struct {
		name        string
		data        map[string]any
		expectError bool
	}{
		{
			name: "all required fields present",
			data: map[string]any{
				keySubtype:       "test",
				"duration_ms":    float64(100),
				keyDurationAPIMs: float64(50),
				keyIsError:       false,
				keyNumTurns:      float64(1),
				keySessionID:     testSessionID,
			},
			expectError: false,
		},
		{
			name: "missing subtype",
			data: map[string]any{
				"duration_ms":    float64(100),
				keyDurationAPIMs: float64(50),
				keyIsError:       false,
				keyNumTurns:      float64(1),
				keySessionID:     testSessionID,
			},
			expectError: true,
		},
		{
			name: "missing duration_ms",
			data: map[string]any{
				keySubtype:       "test",
				keyDurationAPIMs: float64(50),
				keyIsError:       false,
				keyNumTurns:      float64(1),
				keySessionID:     testSessionID,
			},
			expectError: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := &shared.ResultMessage{}
			err := strategy.ParseFields(testCase.data, result)

			if testCase.expectError && err == nil {
				t.Error("expected error, got nil")
			}

			if !testCase.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestOptionalFieldsStrategyAllFields(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{
		"total_cost_usd": float64(0.5),
		"usage":          map[string]any{"tokens": 100},
		fieldResult:      map[string]any{"status": "ok"},
	}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.TotalCostUSD == nil {
		t.Error("expected TotalCostUSD to be set")
	}

	if result.Usage == nil {
		t.Error("expected Usage to be set")
	}

	if result.Result == nil {
		t.Error("expected Result to be set")
	}
}

func TestOptionalFieldsStrategyNoFields(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.TotalCostUSD != nil {
		t.Error("expected TotalCostUSD to be nil")
	}

	if result.Usage != nil {
		t.Error("expected Usage to be nil")
	}

	if result.Result != nil {
		t.Error("expected Result to be nil")
	}
}

func TestOptionalFieldsStrategyPartialFields(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{
		"total_cost_usd": float64(0.5),
	}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.TotalCostUSD == nil {
		t.Error("expected TotalCostUSD to be set")
	}

	if result.Usage != nil {
		t.Error("expected Usage to be nil")
	}

	if result.Result != nil {
		t.Error("expected Result to be nil")
	}
}

// A string-typed `result` is what the CLI actually emits for a plain
// run, for a --json-schema run, and for its own errors. The object-only
// branch dropped all three on the floor.
func TestOptionalFieldsStrategyStringResult(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{fieldResult: `{"verdicts":[]}`}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.ResultText != `{"verdicts":[]}` {
		t.Errorf("ResultText = %q, want the string result", result.ResultText)
	}

	if result.Result != nil {
		t.Error("expected Result to stay nil for a string result")
	}
}

func TestOptionalFieldsStrategyStructuredOutput(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{
		"structured_output": map[string]any{"verdict": "pass"},
	}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	structured, ok := result.StructuredOutput.(map[string]any)
	if !ok {
		t.Fatalf("StructuredOutput = %T, want map[string]any", result.StructuredOutput)
	}

	if structured["verdict"] != "pass" {
		t.Errorf("StructuredOutput = %v, want the schema-conforming object", structured)
	}
}

// An object-typed `result` must keep working exactly as before.
func TestOptionalFieldsStrategyObjectResultStillParses(t *testing.T) {
	strategy := &parser.OptionalFieldsStrategy{}
	data := map[string]any{fieldResult: map[string]any{"answer": "4"}}

	result := &shared.ResultMessage{}

	err := strategy.ParseFields(data, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Result == nil {
		t.Fatal("expected Result to be set for an object result")
	}

	if result.ResultText != "" {
		t.Errorf("ResultText = %q, want empty for an object result", result.ResultText)
	}
}
