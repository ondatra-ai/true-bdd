package architecture

import (
	"fmt"
	"strings"
)

const (
	kindCLI     = "cli"
	kindHTTP    = "http"
	kindUI      = "ui"
	kindLibrary = "library"
)

// PublicInterface declares what a service offers across its process boundary.
type PublicInterface struct {
	references map[string]bool
	Kind       string      `yaml:"kind"`
	Document   string      `yaml:"document,omitempty"`
	Elements   []UIElement `yaml:"elements,omitempty"`
}

// UIElement identifies an element by role and accessible name within a page or region.
type UIElement struct {
	Page   string `yaml:"page,omitempty"`
	Region string `yaml:"region,omitempty"`
	Role   string `yaml:"role"`
	Name   string `yaml:"name"`
}

// Finding explains an invalid declaration; document contents are checked at load time.
func (p *PublicInterface) Finding() string {
	if p == nil {
		return "declares no Public interface"
	}

	switch p.Kind {
	case kindCLI, kindHTTP:
		if strings.TrimSpace(p.Document) == "" {
			return "Public interface requires a document"
		}

		if len(p.Elements) != 0 {
			return "Public interface elements belong to the ui kind"
		}
	case kindUI:
		return p.uiFinding()
	case kindLibrary:
		return "Public interface kind library is excluded: a service must cross a process boundary"
	default:
		return fmt.Sprintf("Public interface kind %q is not supported yet", p.Kind)
	}

	return ""
}

func (p *PublicInterface) uiFinding() string {
	if p.Document != "" || len(p.Elements) == 0 {
		return "Public interface kind ui requires inline elements, without a document"
	}

	seen := make(map[UIElement]bool)

	for _, element := range p.Elements {
		if strings.TrimSpace(element.Role) == "" || strings.TrimSpace(element.Name) == "" ||
			(strings.TrimSpace(element.Page) == "" && strings.TrimSpace(element.Region) == "") {
			return "Public interface ui element requires a role, accessible name and page or region"
		}

		if seen[element] {
			return "Public interface ui elements repeat a role and accessible name within the same page or region"
		}

		seen[element] = true
	}

	return ""
}
