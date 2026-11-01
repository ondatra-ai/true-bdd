package catalogue

// StepDefinition is one step definition a suite registers: its pattern as
// the suite compiles it, and the file and line it is written at.
type StepDefinition struct {
	Pattern string
	File    string
	Line    int
}
