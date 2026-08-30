package alint

// The JSON shapes and the manifest quirk are the two halves of this package
// that answer without spawning; the tests reach them through this seam, which
// the compiler drops from any non-test build.

func Decode(stdout string) (Report, error) { return decode(stdout) }

func ScopeBody(paths []string) string { return scopeBody(paths) }

func ScopeEntries(manifest string) []string { return scopeEntries(manifest) }
