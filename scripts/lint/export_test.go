package lint

// The budget scanner and the manifest parser are unexported; the tests reach
// them through this seam, which the compiler drops from any non-test build.

// Finding is one over-budget run, flattened for assertion.
type Finding struct {
	Line int
	Text string
}

func ScanComments(path, src string, isGo bool) []Finding {
	found := scanComments(path, src, isGo)
	out := make([]Finding, 0, len(found))

	for _, item := range found {
		out = append(out, Finding{Line: item.line, Text: item.text})
	}

	return out
}

func VendoredSkills(manifest string) []string { return vendoredSkills(manifest) }

const (
	MaxProse = maxProse
	MaxBlock = maxBlock
)
