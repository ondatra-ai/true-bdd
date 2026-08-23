package reporter

import (
	"gopkg.in/yaml.v3"
)

// ChecklistPrompt is one entry of the flattened prompt list a run walked.
// SectionName is the checklist author's human name ("Scenario Merge");
// filenames only carry the id. HasFix says the prompt has an `F:` (see turnRef).
type ChecklistPrompt struct {
	SectionID   string
	SectionName string
	HasFix      bool
}

// ChecklistDoc is the run's own checklist, flattened exactly the way the
// engine flattened it — read from the run's tmpdir, not the working tree,
// since only the run's own copy has the prompt indices it actually used (see nameOperations).
type ChecklistDoc struct {
	Path   string
	Loaded bool
	// Prompts is 0-indexed; artifact filenames are 1-indexed. Use
	// Prompt to cross that boundary.
	Prompts []ChecklistPrompt
}

// Prompt returns the entry for a 1-based prompt index, as it appears in an
// artifact filename. Not-ok for a doc that never loaded or an index past
// the end; the caller degrades rather than guessing.
func (d ChecklistDoc) Prompt(index int) (ChecklistPrompt, bool) {
	if index < 1 || index > len(d.Prompts) {
		return ChecklistPrompt{}, false
	}

	return d.Prompts[index-1], true
}

// rawChecklist is the subset of the checklist YAML the report needs. The
// engine's own loader lives under src/internal/ and is unreachable from
// tests/ (Go's internal rule), so the shape is restated here.
type rawChecklist struct {
	Sections []struct {
		ID                string `yaml:"id"`
		Name              string `yaml:"name"`
		ValidationPrompts []struct {
			Skip string `yaml:"skip"`
			// Uppercase on purpose: the checklist format spells the
			// question `Q:` and the fix template `F:`, and the tag has to
			// match the file rather than a naming convention.
			F string `yaml:"F"` //nolint:tagliatelle // the checklist YAML key is literally "F"
		} `yaml:"validation_prompts"`
	} `yaml:"sections"`
}

// loadChecklistDoc reads the checklist the run logged and flattens it,
// matching checklist_loader.go's own walk exactly — see
// TestLoadChecklistDocFlattensLikeTheEngine and TestLoadChecklistDocDegrades.
func loadChecklistDoc(fixtureDir, logged string) ChecklistDoc {
	doc := ChecklistDoc{Path: logged}

	if logged == "" {
		return doc
	}

	// The path comes from the run's own log, so it is refused if it would
	// escape the fixture (see ContainedFile).
	data, ok := ReadContained(fixtureDir, logged)
	if !ok {
		return doc
	}

	var raw rawChecklist

	err := yaml.Unmarshal([]byte(data), &raw)
	if err != nil {
		return doc
	}

	for _, section := range raw.Sections {
		for _, prompt := range section.ValidationPrompts {
			if prompt.Skip != "" {
				continue
			}

			doc.Prompts = append(doc.Prompts, ChecklistPrompt{
				SectionID:   section.ID,
				SectionName: section.Name,
				HasFix:      prompt.F != "",
			})
		}
	}

	doc.Loaded = true

	return doc
}
