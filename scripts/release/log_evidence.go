package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
)

const (
	unavailableValue   = "unavailable"
	configurationStage = "configuration"
)

type logEvidence struct {
	stage    string
	spend    float64
	hasSpend bool
}

func readEvidence(data []byte, outcome *HostResult) (logEvidence, error) {
	evidence := logEvidence{stage: configurationStage}

	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var record map[string]any

		err := json.Unmarshal(line, &record)
		if err != nil {
			return evidence, fmt.Errorf("decoding engine log: %w", err)
		}

		evidence.record(record, line, outcome)
	}

	if evidence.hasSpend {
		outcome.ClaudeSpend = fmt.Sprintf("$%.6f", evidence.spend)
	}

	return evidence, nil
}

func (e *logEvidence) record(record map[string]any, line []byte, outcome *HostResult) {
	switch record["msg"] {
	case "Refusing to start":
		e.stage = refusalStage(record)
	case "Loaded architecture":
		e.stage = "Specification"
	case "Running tests", "Cannot run test suite", "Suite failures":
		e.stage = "suite discovery"
	case enginelog.MsgDispatch:
		e.stage = "AI boundary"

		outcome.Turns = append(outcome.Turns, Turn{Dispatch: json.RawMessage(bytes.Clone(line))})
	case enginelog.MsgUsage:
		if cost, ok := record[enginelog.KeyCostUSD].(float64); ok && record[enginelog.KeyCLI] == "claude" {
			e.spend += cost
			e.hasSpend = true
		}
	default:
		recordDenial(record, line, outcome)
	}
}

func recordDenial(record map[string]any, line []byte, outcome *HostResult) {
	message, _ := record["msg"].(string)
	if !strings.Contains(strings.ToLower(message), "denied") || len(outcome.Turns) == 0 {
		return
	}

	turn := &outcome.Turns[len(outcome.Turns)-1]
	turn.Denials = append(turn.Denials, json.RawMessage(bytes.Clone(line)))

	if _, ok := record["label"]; ok {
		outcome.DenialTargets = "see per-turn denied strings; absent targets unavailable"
	}
}

func refusalStage(record map[string]any) string {
	reason, _ := record["error"].(string)
	if reason == "" {
		return configurationStage
	}

	for _, marker := range []string{"true-bdd init", "template", "checklist", configurationStage} {
		if strings.Contains(strings.ToLower(reason), marker) {
			return configurationStage
		}
	}

	return "Specification"
}
