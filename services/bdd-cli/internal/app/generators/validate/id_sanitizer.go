package validate

import "regexp"

// idSanitizerUnsafe matches every run of characters that must not appear
// in a path segment used for a per-run tmp artifact filename.
var idSanitizerUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sanitizeID flattens a subject or section identifier into a single
// filesystem-safe path segment. Build-code subject ids look like
// "frontend/integration/playwright:<startup>" — the slashes would spawn
// uncreated nested dirs and the ":" / "<>" are invalid on some
// filesystems, so os.WriteFile fails and the debug artifact (and its tmp
// result file) is silently lost. Collapsing every unsafe run to a single
// "-" keeps ids like "INT-900" or "4.1" untouched while making the
// build-code ids writable.
func sanitizeID(id string) string {
	return idSanitizerUnsafe.ReplaceAllString(id, "-")
}
