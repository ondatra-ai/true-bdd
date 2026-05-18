package checklist

// Checklist represents a single-stage validation checklist YAML.
// Structure: sections[] -> validation_prompts[].
type Checklist struct {
	Version     string       `yaml:"version"`
	LastUpdated string       `yaml:"last_updated"`
	DefaultDocs []string     `yaml:"default_docs,omitempty"`
	Config      *ConfigBlock `yaml:"config,omitempty"`
	Sections    []Section    `yaml:"sections"`
}

// ConfigBlock holds per-checklist engine tuning knobs.
type ConfigBlock struct {
	MaxApplyAttempts int `yaml:"max_apply_attempts,omitempty"`
}

// Section represents a validation section within a checklist.
type Section struct {
	ID                string   `yaml:"id"`
	Name              string   `yaml:"name"`
	Description       string   `yaml:"description,omitempty"`
	Source            string   `yaml:"source,omitempty"`
	ValidationPrompts []Prompt `yaml:"validation_prompts"`
}
