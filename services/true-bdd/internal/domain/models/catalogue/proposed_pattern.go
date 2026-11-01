package catalogue

// ProposedPattern is one step definition pattern an AI proposes for a
// concept, as data: the engine checks it before writing any skeleton.
type ProposedPattern struct {
	Pattern  string           `json:"pattern"  yaml:"pattern"`
	Captures []PatternCapture `json:"captures" yaml:"captures"`
	Serves   string           `json:"serves"   yaml:"serves"`
}

// PatternCapture names and explains one capture group, in pattern order.
type PatternCapture struct {
	Name  string `json:"name"  yaml:"name"`
	Means string `json:"means" yaml:"means"`
}
