package validate

import "regexp"

// idSanitizerUnsafe matches every run of characters that must not appear
// in a path segment used for a per-run tmp artifact filename.
var idSanitizerUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sanitizeID flattens a subject or section identifier into a single
// filesystem-safe path segment: build-code ids contain `/` and `:`,
// which break os.WriteFile on some filesystems and silently lose the debug artifact.
func sanitizeID(id string) string {
	return idSanitizerUnsafe.ReplaceAllString(id, "-")
}
