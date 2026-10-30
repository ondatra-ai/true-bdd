package architecture

import (
	"fmt"
	"strings"
)

// InterfaceReference names one command, extension, HTTP operation or scoped UI element.
type InterfaceReference struct {
	Command      string     `yaml:"command,omitempty"`
	File         string     `yaml:"file,omitempty"`
	LogRecord    string     `yaml:"log_record,omitempty"`
	OutgoingCall string     `yaml:"outgoing_call,omitempty"`
	OperationID  string     `yaml:"operation_id,omitempty"`
	Element      *UIElement `yaml:"element,omitempty"`
}

// String names the reference in a finding, including malformed combinations.
func (r InterfaceReference) String() string { return fmt.Sprint(r.selectors()) }

// Selector names the reference as its document key and value, as in "command: halve".
func (r InterfaceReference) Selector() string { return strings.Join(r.selectors(), ", ") }

// Declares checks a reference against the loaded declaration and document.
func (p *PublicInterface) Declares(reference InterfaceReference) bool {
	if p == nil || p.Finding() != "" || len(reference.selectors()) != 1 {
		return false
	}

	if p.Kind == kindUI && reference.Element != nil {
		for _, element := range p.Elements {
			if element == *reference.Element {
				return true
			}
		}
	}

	return p.references[reference.String()]
}

func (r InterfaceReference) selectors() []string {
	selectors := make([]string, 0)

	for _, pair := range []struct{ key, value string }{
		{"command", r.Command}, {"file", r.File}, {"log_record", r.LogRecord},
		{"outgoing_call", r.OutgoingCall}, {"operation_id", r.OperationID},
	} {
		if pair.value != "" {
			selectors = append(selectors, pair.key+": "+pair.value)
		}
	}

	if r.Element != nil {
		selectors = append(selectors, fmt.Sprintf("element: %+v", *r.Element))
	}

	return selectors
}
