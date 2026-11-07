package architecture

// Dependency declares a process the service calls and the recording rules its requests need.
type Dependency struct {
	Name       string    `yaml:"name"`
	Protocol   string    `yaml:"protocol"`
	Scope      string    `yaml:"scope"`
	Recording  Recording `yaml:"recording,omitempty"`
	Invocation struct {
		Binary string `yaml:"binary"`
	} `yaml:"invocation"`
}

// Recording states request normalization, recording-only exclusions, unread paths and a read-only marker.
type Recording struct {
	Placeholders []Placeholder     `yaml:"placeholders,omitempty"`
	Unrecorded   []string          `yaml:"unrecorded,omitempty"`
	Unread       []string          `yaml:"unread,omitempty"`
	ReadOnlyWhen map[string]string `yaml:"read_only_when,omitempty"`
}

// Placeholder replaces matching request fragments and resolves their current directory beneath Directory.
type Placeholder struct {
	Token     string `yaml:"token"`
	Pattern   string `yaml:"pattern"`
	Directory string `yaml:"directory"`
}

// CLIDependencies selects only vendor CLIs; owned and network services run through their own protocols.
func (s Service) CLIDependencies() []Dependency {
	var selected []Dependency

	for _, dependency := range s.Dependencies {
		if dependency.Protocol == "cli" && dependency.Scope == "third_party" {
			selected = append(selected, dependency)
		}
	}

	return selected
}

// Binary is the executable declared by invocation, falling back to the dependency's name.
func (d Dependency) Binary() string {
	if d.Invocation.Binary != "" {
		return d.Invocation.Binary
	}

	return d.Name
}
