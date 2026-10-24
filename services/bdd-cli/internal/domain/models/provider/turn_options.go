package provider

// TurnOptions controls execution independently of tool permissions.
type TurnOptions struct {
	// WatchWriterResponse times open model responses for build code's crush writer.
	WatchWriterResponse bool
	// CodexWriter confines only build code's codex apply turn.
	CodexWriter bool
}
