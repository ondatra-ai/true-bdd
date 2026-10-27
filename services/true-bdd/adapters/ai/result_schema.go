package ai

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"

// SupportsResultSchema reports whether a CLI can enforce a JSON Schema on
// its answer — claude via `--json-schema`, codex via `--output-schema`.
func SupportsResultSchema(cli provider.CLI) bool {
	switch cli {
	case provider.CLIClaude, provider.CLICodex:
		return true
	default:
		return false
	}
}
