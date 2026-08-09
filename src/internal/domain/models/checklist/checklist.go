package checklist

// Checklist represents a single-stage validation checklist YAML.
// Structure: sections[] -> validation_prompts[].
type Checklist struct {
	Version     string       `yaml:"version"`
	LastUpdated string       `yaml:"last_updated"`
	DefaultDocs []string     `yaml:"default_docs,omitempty"`
	Config      *ConfigBlock `yaml:"config,omitempty"`
	Engine      *EngineBlock `yaml:"engine,omitempty"`
	Sections    []Section    `yaml:"sections"`
}

// ConfigBlock holds per-checklist engine tuning knobs.
type ConfigBlock struct {
	MaxApplyAttempts int `yaml:"max_apply_attempts,omitempty"`
}

// EngineBlock selects a model tier per AI role in a checklist cell.
// Each value names a tier declared under `engine.models` in
// true-bdd.yaml (xhigh / high / coder), never a model id — which model
// a tier means is the host project's decision, not the checklist's.
//
// An empty field falls through to `engine.default_model`. An
// individual prompt overrides any of these; see Prompt.
type EngineBlock struct {
	// PromptModel runs the validation turn that answers `Q:`.
	PromptModel string `yaml:"prompt_model,omitempty"`
	// FixModel runs the turn that converts a failure into a fix prompt.
	FixModel string `yaml:"fix_model,omitempty"`
	// ApplyModel runs the turn that writes the fix to disk.
	ApplyModel string `yaml:"apply_model,omitempty"`
}

// Section represents a validation section within a checklist.
type Section struct {
	ID                string   `yaml:"id"`
	Name              string   `yaml:"name"`
	Description       string   `yaml:"description,omitempty"`
	Source            string   `yaml:"source,omitempty"`
	ValidationPrompts []Prompt `yaml:"validation_prompts"`
}
