package cli

// Dependency names a CLI executable and the host's recording contract for its requests.
type Dependency struct {
	// Name is the dependency executable basename.
	Name string `json:"name"`
	// Recording describes the host-specific normalization and snapshot rules.
	Recording Recording `json:"recording"`
}

// Recording declares volatile fragments, recording-only exclusions, unread paths and a read-only environment marker.
type Recording struct {
	// Placeholders replace volatile request and response fragments with stable tokens.
	Placeholders []Placeholder `json:"placeholders,omitempty"`
	// Unrecorded names paths excluded from per-call recordings but visible in run observations.
	Unrecorded []string `json:"unrecorded,omitempty"`
	// Unread names paths neither run nor recording snapshots may read.
	Unread []string `json:"unread,omitempty"`
	// ReadOnlyWhen identifies read-only calls by an environment key and value.
	ReadOnlyWhen map[string]string `json:"read_only_when,omitempty"`
}

// Placeholder normalizes matching text to Token and resolves the newest matching directory beneath Directory.
type Placeholder struct {
	// Token is the stable replacement for matching text.
	Token string `json:"token"`
	// Pattern matches volatile text in requests and responses.
	Pattern string `json:"pattern"`
	// Directory locates the parent whose newest matching child resolves Token.
	Directory string `json:"directory"`
}
