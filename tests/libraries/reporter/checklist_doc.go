package reporter

import (
	"gopkg.in/yaml.v3"
)

// ChecklistPrompt is one entry of the flattened prompt list a run walked.
//
// SectionName is the human name the checklist author gave the section
// ("Scenario Merge"); the artifact filenames only ever carry the id
// ("merge"), so this is the only place the readable name exists.
//
// HasFix says the prompt declares an `F:` fix template. That matters for
// naming a turn: the validation prompt embeds F and asks the model to
// render a fix_prompt from it, so a prompt WITH an F was answered
// against Q and F both, and one without was answered against Q alone.
type ChecklistPrompt struct {
	SectionID   string
	SectionName string
	HasFix      bool
}

// ChecklistDoc is the run's own checklist, flattened exactly the way the
// engine flattened it.
//
// Read from the run's tmpdir rather than from the working tree: a
// fixture's `checklist_prompts:` filter generates a reduced checklist
// into the tmpdir (runner/checklist_filter.go), and a fixture may ship
// its own override under input/true-bdd/. Only the run's own copy has
// the prompt indices the run actually used.
type ChecklistDoc struct {
	Path   string
	Loaded bool
	// Prompts is 0-indexed; artifact filenames are 1-indexed. Use
	// Prompt to cross that boundary.
	Prompts []ChecklistPrompt
}

// Prompt returns the entry for a 1-based prompt index, as it appears in
// an artifact filename. Not-ok for a doc that never loaded or an index
// past the end — both are ordinary for a session recorded before this
// file existed, and the caller degrades rather than guessing.
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

// loadChecklistDoc reads the checklist the run logged and flattens it.
//
// The flattening MUST match checklist_loader.go:73 — sections in file
// order, prompts in file order, skipping any prompt with a non-empty
// `skip:`. That walk is what assigns the %02d index in every artifact
// filename, so a different order here would mislabel every turn.
//
// A missing or unreadable file yields an unloaded doc. The label then
// falls back to the raw section id from the filename, which is what
// every session recorded before these records existed will do.
func loadChecklistDoc(fixtureDir, logged string) ChecklistDoc {
	doc := ChecklistDoc{Path: logged}

	if logged == "" {
		return doc
	}

	// The path comes out of the run's own log, which is a file on disk
	// like any other. An absolute path, or a relative one that climbs out
	// of the fixture, would make loading a report read YAML from anywhere
	// on the machine and surface its section names in the UI. Neither
	// shape is one the engine ever writes, so both are simply refused.
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
