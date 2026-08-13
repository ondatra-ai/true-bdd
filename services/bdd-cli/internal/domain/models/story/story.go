package story

type Story struct {
	ID                 string                `json:"id"                  yaml:"id"`
	Title              string                `json:"title"               yaml:"title"`
	AsA                string                `json:"as_a"                yaml:"as_a"`
	IWant              string                `json:"i_want"              yaml:"i_want"`
	SoThat             string                `json:"so_that"             yaml:"so_that"`
	Status             string                `json:"status"              yaml:"status"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria" yaml:"acceptance_criteria"`
}
