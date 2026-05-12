package story

type QAResults struct {
	ReviewDate    string     `json:"review_date"    yaml:"review_date"`
	ReviewedBy    string     `json:"reviewed_by"    yaml:"reviewed_by"`
	Assessment    Assessment `json:"assessment"     yaml:"assessment"`
	GateStatus    string     `json:"gate_status"    yaml:"gate_status"`
	GateReference string     `json:"gate_reference" yaml:"gate_reference"`
}
