package story

type AcceptanceCriterion struct {
	ID          string         `json:"id"              yaml:"id"`
	Description string         `json:"description"     yaml:"description"`
	Steps       []ScenarioStep `json:"steps,omitempty" yaml:"steps,omitempty"`
}
