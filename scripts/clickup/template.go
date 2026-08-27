package clickup

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// The ticket shape, a file so wording is a diff on prose, not Go literals.
//
//go:embed ticket.yaml
var ticketYAML []byte

// bodyLimit caps how much of a finding reaches the ticket.
const bodyLimit = 4000

// heading is one `### ` section of a ticket.
type heading struct {
	Name string `yaml:"name"`
	Body string `yaml:"body"`
}

// ticketShape is ticket.yaml, decoded.
type ticketShape struct {
	Version    int       `yaml:"version"`
	Status     string    `yaml:"status"`
	StatusRule string    `yaml:"status_rule"`
	Headings   []heading `yaml:"headings"`
}

// shape decodes the embedded template. A failure here is a broken build, not
// a runtime condition: the file ships inside the binary, so if it does not
// parse, nothing this package does can be trusted to have the right shape.
func shape() ticketShape {
	var decoded ticketShape

	err := yaml.Unmarshal(ticketYAML, &decoded)
	if err != nil {
		panic(fmt.Sprintf("scripts/clickup/ticket.yaml does not parse: %v", err))
	}

	return decoded
}

// ticketStatus is where every filed ticket lands, and statusRule is the
// sentence both filing turns carry to say so.
func ticketStatus() string {
	return shape().Status
}

func statusRule() string {
	return strings.TrimSpace(shape().StatusRule)
}

// findingView is what a heading's template renders against: the finding, with
// every absent field already resolved to what the ticket should show.
type findingView struct {
	Origin   string
	Score    int
	Reason   string
	File     string
	Line     string
	Body     string
	Severity string
	Source   string
}

// viewOf resolves what the ticket shows. A queue row can be short of any
// field, and `?` in a heading is readable where an empty line is not.
func viewOf(finding Finding, origin string) findingView {
	reason := strings.TrimSpace(finding.Reason)
	if reason == "" {
		reason = "(no reason recorded)"
	}

	return findingView{
		Origin:   origin,
		Score:    finding.Score,
		Reason:   reason,
		File:     orUnknown(finding.File),
		Line:     orUnknown(finding.Line),
		Body:     textutil.Truncate(strings.TrimSpace(finding.Body), bodyLimit),
		Severity: orUnknown(finding.Severity),
		Source:   orUnknown(finding.Source),
	}
}

// renderHeadings turns one finding into its four `### ` sections.
func renderHeadings(finding Finding, origin string) []string {
	view := viewOf(finding, origin)

	sections := shape().Headings

	const linesPerHeading = 3

	lines := make([]string, 0, len(sections)*linesPerHeading)

	for _, section := range sections {
		lines = append(lines, "### "+section.Name, "", expand(section, view))
	}

	return lines
}

// expand runs one heading's template. A template that does not parse or
// execute is the same broken build as an unparseable file.
func expand(section heading, view findingView) string {
	parsed, err := template.New(section.Name).Parse(section.Body)
	if err != nil {
		panic(fmt.Sprintf("ticket.yaml heading %q does not parse: %v", section.Name, err))
	}

	var out strings.Builder

	err = parsed.Execute(&out, view)
	if err != nil {
		panic(fmt.Sprintf("ticket.yaml heading %q does not render: %v", section.Name, err))
	}

	// Not trimmed: a `|` block ends in exactly one newline, and that newline
	// is the blank line between this heading and the next.
	return out.String()
}
