package story

// Story is one user story. ID is a slug (`scen-check`, `96.2`); Feature is
// the optional label grouping it with related Stories, omitted when empty so
// a Story without one renders exactly as it did before the label existed.
type Story struct {
	ID                 string                `json:"id"                  yaml:"id"`
	Feature            string                `json:"feature,omitempty"   yaml:"feature,omitempty"`
	Title              string                `json:"title"               yaml:"title"`
	AsA                string                `json:"as_a"                yaml:"as_a"`
	IWant              string                `json:"i_want"              yaml:"i_want"`
	SoThat             string                `json:"so_that"             yaml:"so_that"`
	Status             string                `json:"status"              yaml:"status"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria" yaml:"acceptance_criteria"`
}
