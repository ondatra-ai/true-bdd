package template

// FixDocument is one document a `scen check` fix may write, through its
// scratch copy: the applier edits Scratch, and the engine replaces Path
// with it once it parses. Key is the document's `docs:` key.
type FixDocument struct {
	Key     string
	Path    string
	Scratch string
}
