package parser

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// FieldParsingStrategy defines the interface for parsing message fields.
type FieldParsingStrategy interface {
	ParseFields(data map[string]any, result *shared.ResultMessage) error
}

// RequiredFieldsStrategy parses and validates required fields.
type RequiredFieldsStrategy struct{}

// ParseFields parses all required fields for a result message.
func (s *RequiredFieldsStrategy) ParseFields(data map[string]any, result *shared.ResultMessage) error {
	validators := []func(map[string]any, *shared.ResultMessage) error{
		validateSubtype,
		validateDurationMS,
		validateDurationAPIMS,
		validateIsError,
		validateNumTurns,
		validateSessionID,
	}

	for _, validator := range validators {
		err := validator(data, result)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateSubtype validates the subtype field.
func validateSubtype(data map[string]any, result *shared.ResultMessage) error {
	if subtype, ok := data["subtype"].(string); ok {
		result.Subtype = subtype

		return nil
	}

	return shared.NewMessageParseError("result message missing subtype field", data)
}

// validateDurationMS validates the duration_ms field.
func validateDurationMS(data map[string]any, result *shared.ResultMessage) error {
	if durationMS, ok := data["duration_ms"].(float64); ok {
		result.DurationMs = int(durationMS)

		return nil
	}

	return shared.NewMessageParseError("result message missing or invalid duration_ms field", data)
}

// validateDurationAPIMS validates the duration_api_ms field.
func validateDurationAPIMS(data map[string]any, result *shared.ResultMessage) error {
	if durationAPIMS, ok := data["duration_api_ms"].(float64); ok {
		result.DurationAPIMs = int(durationAPIMS)

		return nil
	}

	return shared.NewMessageParseError("result message missing or invalid duration_api_ms field", data)
}

// validateIsError validates the is_error field.
func validateIsError(data map[string]any, result *shared.ResultMessage) error {
	if isError, ok := data["is_error"].(bool); ok {
		result.IsError = isError

		return nil
	}

	return shared.NewMessageParseError("result message missing or invalid is_error field", data)
}

// validateNumTurns validates the num_turns field.
func validateNumTurns(data map[string]any, result *shared.ResultMessage) error {
	if numTurns, ok := data["num_turns"].(float64); ok {
		result.NumTurns = int(numTurns)

		return nil
	}

	return shared.NewMessageParseError("result message missing or invalid num_turns field", data)
}

// validateSessionID validates the session_id field.
func validateSessionID(data map[string]any, result *shared.ResultMessage) error {
	if sessionID, ok := data["session_id"].(string); ok {
		result.SessionID = sessionID

		return nil
	}

	return shared.NewMessageParseError("result message missing session_id field", data)
}

// OptionalFieldsStrategy parses optional fields.
type OptionalFieldsStrategy struct{}

// ParseFields parses all optional fields for a result message.
func (s *OptionalFieldsStrategy) ParseFields(data map[string]any, result *shared.ResultMessage) error {
	if totalCostUSD, ok := data["total_cost_usd"].(float64); ok {
		result.TotalCostUSD = &totalCostUSD
	}

	if usage, ok := data["usage"].(map[string]any); ok {
		result.Usage = &usage
	}

	if resultData, ok := data["result"]; ok {
		if resultMap, ok := resultData.(map[string]any); ok {
			result.Result = &resultMap
		}
	}

	return nil
}
