package commit

// The sweep table is unexported; the tests reach it through this seam, which
// the compiler drops from any non-test build.

// Sweep names one sweep by its report line, so a test can pick it out.
func Sweep(report string) func([]string) []string {
	for _, check := range sweeps {
		if check.report == report {
			return check.hits
		}
	}

	return nil
}

// Reports is every sweep's report line, in table order.
func Reports() []string {
	out := make([]string, 0, len(sweeps))
	for _, check := range sweeps {
		out = append(out, check.report)
	}

	return out
}

const RecordingsGlob = recordingsGlob

// SanitizeBranchName is the branch step's repair of the model's answer.
func SanitizeBranchName(answer string) string {
	return sanitizeBranchName(answer)
}

// PanicStop unwinds the way dief and usage do, so a test can prove guard
// converts a stop and re-panics anything else.
func PanicStop(message string) { panic(stopSentinel{message: message}) }

// Guard is the recover-to-error wrapper Execute and Embed are built on.
func Guard(body func()) error { return guard(body) }
