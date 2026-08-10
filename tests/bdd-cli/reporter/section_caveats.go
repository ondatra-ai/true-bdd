package reporter

import _ "embed"

// caveatsSection states what the numbers do and do not cover. Kept as an
// asset because it is prose about method, edited far more often than the
// code that emits it.
//
//go:embed caveats.html
var caveatsSection string

// writeCaveats closes the report with its own limits.
func (r *Renderer) writeCaveats() {
	r.write("\n", caveatsSection)
}
