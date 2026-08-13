package story

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"

// StoryDocument is the in-memory representation of a `docs/product/stories/<id>-*.yaml`
// file. Today only the `story:` block is read after load; legacy sections
// (`change_log`, `qa_results`, `dev_agent_record`, `scenarios`) used by the
// pre-true-bdd era have been dropped. ArchitectureDocs is populated by the
// container, not the YAML.
type StoryDocument struct {
	Story            Story                  `json:"story" yaml:"story"`
	ArchitectureDocs *docs.ArchitectureDocs `json:"-"     yaml:"-"`
}
