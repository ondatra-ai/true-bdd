package story

import "bdd-cli/src/internal/infrastructure/docs"

// StoryDocument is the in-memory representation of a `docs/stories/<id>-*.yaml`
// file. Today only the `story:` block is read after load; legacy sections
// (`change_log`, `qa_results`, `dev_agent_record`, `scenarios`) used by the
// pre-bdd-cli era have been dropped. ArchitectureDocs is populated by the
// container, not the YAML.
type StoryDocument struct {
	Story            Story                  `json:"story" yaml:"story"`
	ArchitectureDocs *docs.ArchitectureDocs `json:"-"     yaml:"-"`
}
