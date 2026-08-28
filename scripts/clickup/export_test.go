package clickup

import (
	"encoding/json"
	"fmt"
)

// The field plan is unexported and reaches the filing turn as JSON, so the
// test asserts against those bytes through this export_test.go seam, which
// the compiler drops from any non-test build.

// PlanFieldsForTest is the plan the filing prompt carries, as it is embedded.
func PlanFieldsForTest(queue []Finding) []byte {
	encoded, err := json.MarshalIndent(planFields(queue), "", "  ")
	if err != nil {
		panic(err)
	}

	return encoded
}

// DropAlreadyOpenForTest is the dedupe filter FileDeduped runs before the
// render; the listing turn it feeds on cannot run in a test.
func DropAlreadyOpenForTest(queue []Finding, open []Task) ([]Finding, []Task) {
	return dropAlreadyOpen(queue, open)
}

// MisplacedForTest is the backlog check report runs; the filing turn that
// produces the statuses it reads cannot run in a test.
func MisplacedForTest(filed []Ticket) []string {
	return misplaced(filed)
}

// CountHeadingsForTest is the count FileDocument tells the filing turn to
// create; the turn itself cannot run in a test.
func CountHeadingsForTest(document string) int {
	return countHeadings(document)
}

// DocumentPromptForTest is the document turn's prompt, as it is sent.
func DocumentPromptForTest(document, tag string) string {
	return fmt.Sprintf(documentPromptTemplate,
		countHeadings(document), listID(), ticketStatus(), tag, statusRule(), document)
}

// TicketStatusForTest is the status ticket.yaml declares, and
// StatusRuleForTest the sentence both filing turns carry to state it.
func TicketStatusForTest() string {
	return ticketStatus()
}

func StatusRuleForTest() string {
	return statusRule()
}

// HeadingNamesForTest is the heading order ticket.yaml declares.
func HeadingNamesForTest() []string {
	names := make([]string, 0, len(shape().Headings))
	for _, section := range shape().Headings {
		names = append(names, section.Name)
	}

	return names
}
