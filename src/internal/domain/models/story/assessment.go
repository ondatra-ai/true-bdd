package story

type Assessment struct {
	Summary                    string   `json:"summary"                      yaml:"summary"`
	Strengths                  []string `json:"strengths"                    yaml:"strengths"`
	Improvements               []string `json:"improvements"                 yaml:"improvements"`
	RiskLevel                  string   `json:"risk_level"                   yaml:"risk_level"`
	RiskReason                 string   `json:"risk_reason"                  yaml:"risk_reason"`
	TestabilityScore           int      `json:"testability_score"            yaml:"testability_score"`
	TestabilityMax             int      `json:"testability_max"              yaml:"testability_max"`
	TestabilityNotes           string   `json:"testability_notes"            yaml:"testability_notes"`
	ImplementationReadiness    int      `json:"implementation_readiness"     yaml:"implementation_readiness"`
	ImplementationReadinessMax int      `json:"implementation_readiness_max" yaml:"implementation_readiness_max"`
}
