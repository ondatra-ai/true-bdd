package story

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"

// StoryDocument is the in-memory representation of a
// `docs/product/stories/<id>-*.yaml` file. Only the `story:` block is read;
// legacy sections are dropped, and ArchitectureDocs is populated by the container, not the YAML.
type StoryDocument struct {
	Story            Story                  `json:"story" yaml:"story"`
	ArchitectureDocs *docs.ArchitectureDocs `json:"-"     yaml:"-"`
}
