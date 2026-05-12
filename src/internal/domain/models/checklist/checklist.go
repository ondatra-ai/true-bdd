package checklist

// Checklist represents a single-stage validation checklist YAML.
// Structure: sections[] -> validation_prompts[].
type Checklist struct {
	Version     string    `yaml:"version"`
	LastUpdated string    `yaml:"last_updated"`
	DefaultDocs []string  `yaml:"default_docs,omitempty"`
	Sections    []Section `yaml:"sections"`
}

// Section represents a validation section within a checklist.
type Section struct {
	ID                string   `yaml:"id"`
	Name              string   `yaml:"name"`
	Description       string   `yaml:"description,omitempty"`
	Source            string   `yaml:"source,omitempty"`
	ValidationPrompts []Prompt `yaml:"validation_prompts"`
}
