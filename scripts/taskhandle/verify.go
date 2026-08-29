package taskhandle

import (
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

// Detail is the Ticket as read. Aliased so the pure checks below can be tested
// without reaching ClickUp.
type Detail = clickup.Detail

// readyStatus is the only status a Ticket may be taken from. PROCESSING, DONE
// and FAILED are somebody's work, not a fresh start.
const readyStatus = "TO DO"

const shaLength = 40

// check is one requirement and how to read whether this Ticket meets it.
type check struct {
	name string
	met  func(Detail) bool
}

//nolint:gochecknoglobals // the field table ticket-schema.yaml declares.
var fieldChecks = []check{
	{"Good For Agent", func(d Detail) bool { return d.GoodForAgent }},
	{"Triage Score", func(d Detail) bool { return validScore(d.TriageScore) }},
	{"Triage Date", func(d Detail) bool { return strings.TrimSpace(d.TriageDate) != "" }},
	{"Triage Commit", func(d Detail) bool { return len(strings.TrimSpace(d.TriageCommit)) == shaLength }},
	{"Expected Changes", func(d Detail) bool { return strings.TrimSpace(d.ExpectedChanges) != "" }},
}

// verify names every requirement this Ticket does not meet: the status, the
// five fields, and each of the four headings its body must carry.
func verify(detail Detail, headings []string) []string {
	var missing []string

	if !strings.EqualFold(strings.TrimSpace(detail.Status), readyStatus) {
		missing = append(missing,
			"status is "+quote(detail.Status)+", want "+quote(readyStatus))
	}

	for _, one := range fieldChecks {
		if !one.met(detail) {
			missing = append(missing, "field "+quote(one.name)+" is empty or invalid")
		}
	}

	for _, heading := range headings {
		if !hasHeading(detail.Description, heading) {
			missing = append(missing, "heading "+quote("### "+heading)+" is absent")
		}
	}

	return missing
}

// validScore holds the Triage Score to the band the dropdown offers, which is
// also the rubric's.
func validScore(raw string) bool {
	const floor, ceiling = 1, 10

	score, err := strconv.Atoi(strings.TrimSpace(raw))

	return err == nil && score >= floor && score <= ceiling
}

// hasHeading reads the body a line at a time: a heading named inside a
// paragraph is prose, not a section.
func hasHeading(body, name string) bool {
	want := "### " + name

	for _, line := range strings.Split(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), want) {
			return true
		}
	}

	return false
}

func quote(value string) string { return `"` + value + `"` }
