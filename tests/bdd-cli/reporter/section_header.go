package reporter

// writeHeader writes the page title and the run's one-line identity.
func (r *Renderer) writeHeader() {
	total := itoa(len(r.fixtures))

	r.write("\n<header>\n  <h1>TrueBDD fixture suite — run report</h1>\n",
		`  <p class="sub">Session <code>`, escapeHTML(r.session), "</code> ·\n     ",
		itoa(r.passed), "/", total, " passed</p>\n</header>\n")
}
