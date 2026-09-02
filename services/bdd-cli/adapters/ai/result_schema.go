package ai

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"

// SupportsResultSchema reports whether a CLI can enforce a JSON Schema on
// its answer — claude via `--json-schema`, codex via `--output-schema`.
// crush offers neither, so its turns keep the delimited FILE contract.
func SupportsResultSchema(cli provider.CLI) bool {
	switch cli {
	case provider.CLIClaude, provider.CLICodex:
		return true
	case provider.CLICrush:
		return false
	default:
		return false
	}
}
