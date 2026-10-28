package architecture

// PhraseMapping states what a step does through its service's Public interface.
type PhraseMapping struct {
	Template   string               `yaml:"template"`
	Service    string               `yaml:"service"`
	References []InterfaceReference `yaml:"references,omitempty"`
	Does       string               `yaml:"does,omitempty"`
	Intent     string               `yaml:"intent,omitempty"`
}
