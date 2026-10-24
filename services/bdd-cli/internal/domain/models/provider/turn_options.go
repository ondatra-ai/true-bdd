package provider

// TurnOptions controls execution independently of tool permissions.
type TurnOptions struct {
 // ClaudeWriter requests the isolated, unbounded build-code applier.
 ClaudeWriter bool
	// WatchWriterResponse times open model responses for build code's crush writer.
	WatchWriterResponse bool
}
