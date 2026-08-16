package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrCmdRequired is returned when a scenario's When step named no
// command. The runner has nothing to exec without one, and defaulting
// would run something the scenario never asked for.
var ErrCmdRequired = errors.New("scenario named no command to run")

// ErrFilterStemMismatch is returned when a checklist_prompts stem does
// not match the checklist the fixture's cmd actually invokes — a dead
// filter would silently leave the real walk unfiltered.
var ErrFilterStemMismatch = errors.New(
	"checklist_prompts: stem does not match the checklist invoked by cmd")

// ErrFilterConflict is returned when a fixture both declares
// checklist_prompts for a stem and ships an input override for the same
// checklist file — two competing sources of truth.
var ErrFilterConflict = errors.New(
	"checklist_prompts: fixture also ships an input override for this checklist")

// ErrSnippetUnmatched is returned when a snippet matches no prompt —
// the shipped Q was reworded or the snippet is wrong.
var ErrSnippetUnmatched = errors.New("checklist_prompts: snippet matches no prompt")

// ErrSnippetAmbiguous is returned when a snippet matches more than one
// prompt.
var ErrSnippetAmbiguous = errors.New("checklist_prompts: snippet matches multiple prompts")

// ErrSnippetDuplicate is returned when two snippets resolve to the same
// prompt.
var ErrSnippetDuplicate = errors.New("checklist_prompts: two snippets resolve to the same prompt")

// ErrSnippetEmpty is returned for a blank snippet or an empty list.
var ErrSnippetEmpty = errors.New("checklist_prompts: empty snippet list or blank snippet")

// ErrChecklistShape is returned when the target checklist lacks the
// document/sections structure the filter needs.
var ErrChecklistShape = errors.New("checklist_prompts: checklist has no sections to filter")

// ErrUnsupportedYAML is returned for checklist YAML features the
// filter deliberately refuses: anchors/aliases and merge keys (pruning
// could orphan an alias and emit an unparsable file) and multi-document
// streams (only the first document would survive).
var ErrUnsupportedYAML = errors.New(
	"checklist_prompts: checklist uses YAML aliases, merge keys, or multiple documents — not supported")

// ErrFilterDeclaredEmpty is returned when checklist_prompts is present
// in fixture.yaml but empty or null — a declared filter must select
// something, otherwise the fixture silently runs the full checklist.
var ErrFilterDeclaredEmpty = errors.New(
	"checklist_prompts: declared but empty — remove the key or declare a selection")

// checklistFilePerm is the mode of the regenerated checklist file.
const checklistFilePerm = 0o644

// validateChecklistFilters checks the load-time invariants: every
// declared stem matches the invoked checklist and does not collide with
// an input override shipped by the same fixture.
func validateChecklistFilters(fixture *Fixture) error {
	if len(fixture.ChecklistPrompts) == 0 {
		return nil
	}

	invoked := commandChecklistStem(fixture.Cmd)

	for stem, snippets := range fixture.ChecklistPrompts {
		if stem != invoked {
			return fmt.Errorf("%w: declared %q, cmd %q invokes %q",
				ErrFilterStemMismatch, stem, fixture.Cmd, invoked)
		}

		if len(snippets) == 0 {
			return fmt.Errorf("%w: stem %s", ErrSnippetEmpty, stem)
		}

		override := filepath.Join(fixture.Dir, fixture.InputPath,
			"true-bdd", "checklists", stem+".yaml")

		_, statErr := os.Stat(override)
		if statErr == nil {
			return fmt.Errorf("%w: %s", ErrFilterConflict, override)
		}
	}

	return nil
}

// applyChecklistFilters rewrites each declared checklist inside the
// tmpdir to contain only the prompts selected by the fixture's
// snippets. Runs after the input overlay and before the pre-run
// snapshot, so the generated file never pollutes the judge diff.
func applyChecklistFilters(fixture *Fixture, tmpDir string) error {
	for stem, snippets := range fixture.ChecklistPrompts {
		path := filepath.Join(tmpDir, "true-bdd", "checklists", stem+".yaml")

		err := FilterChecklistFile(path, snippets)
		if err != nil {
			return fmt.Errorf("filtering %s for fixture %s: %w", path, fixture.Name, err)
		}
	}

	return nil
}

// commandChecklistStem hyphenates the leading command words of a CLI
// invocation ("us refine 96.5 --fix" -> "us-refine"), mirroring the
// production checklist resolution convention.
func commandChecklistStem(cmd string) string {
	words := make([]string, 0)

	for _, field := range strings.Fields(cmd) {
		if strings.HasPrefix(field, "-") || (field != "" && field[0] >= '0' && field[0] <= '9') {
			break
		}

		words = append(words, field)
	}

	return strings.Join(words, "-")
}

// FilterChecklistFile loads a checklist YAML, keeps only the prompts
// matched by the snippets, and writes it back. The document is edited
// as a yaml.Node tree so everything else — the config block, section
// metadata, and the surviving prompts' exact text — round-trips
// unchanged in value.
//
// Each snippet must match exactly one live (non-skipped) prompt, and
// two snippets must not resolve to the same prompt; violations return
// the exported ErrSnippet* / ErrChecklistShape / ErrUnsupportedYAML
// errors. Exported for reuse by the harness fixture materializer,
// which applies the identical filtering semantics to its fixtures.
func FilterChecklistFile(path string, snippets []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading checklist: %w", err)
	}

	doc, err := parseSingleDocument(data)
	if err != nil {
		return err
	}

	err = rejectAliases(doc)
	if err != nil {
		return err
	}

	err = filterDocument(doc, snippets)
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshaling filtered checklist: %w", err)
	}

	err = os.WriteFile(path, out, checklistFilePerm)
	if err != nil {
		return fmt.Errorf("writing filtered checklist: %w", err)
	}

	return nil
}

// filterDocument mutates the parsed document in place, keeping only the
// selected prompts and dropping sections left without any.
func filterDocument(doc *yaml.Node, snippets []string) error {
	root := documentRoot(doc)
	if root == nil {
		return fmt.Errorf("%w: no document root", ErrChecklistShape)
	}

	sections := mappingValue(root, "sections")
	if sections == nil || sections.Kind != yaml.SequenceNode {
		return fmt.Errorf("%w: no sections sequence", ErrChecklistShape)
	}

	matched, err := selectPrompts(sections, snippets)
	if err != nil {
		return err
	}

	kept := make([]*yaml.Node, 0, len(sections.Content))

	for _, section := range sections.Content {
		prompts := mappingValue(section, "validation_prompts")
		if prompts == nil {
			continue
		}

		selected := make([]*yaml.Node, 0)

		for _, prompt := range prompts.Content {
			if matched[prompt] {
				selected = append(selected, prompt)
			}
		}

		if len(selected) == 0 {
			continue
		}

		prompts.Content = selected

		kept = append(kept, section)
	}

	sections.Content = kept

	return nil
}

// selectPrompts resolves every snippet to exactly one prompt node
// across all sections. Each snippet must match one prompt; two snippets
// must not land on the same prompt.
func selectPrompts(sections *yaml.Node, snippets []string) (map[*yaml.Node]bool, error) {
	matched := map[*yaml.Node]bool{}

	for _, rawSnippet := range snippets {
		snippet := collapseSpaces(rawSnippet)
		if snippet == "" {
			return nil, ErrSnippetEmpty
		}

		hits := findPromptsBySnippet(sections, snippet)

		switch {
		case len(hits) == 0:
			return nil, fmt.Errorf("%w: %q", ErrSnippetUnmatched, rawSnippet)
		case len(hits) > 1:
			return nil, fmt.Errorf("%w: %q matches %d prompts", ErrSnippetAmbiguous, rawSnippet, len(hits))
		case matched[hits[0]]:
			return nil, fmt.Errorf("%w: %q", ErrSnippetDuplicate, rawSnippet)
		}

		matched[hits[0]] = true
	}

	return matched, nil
}

// findPromptsBySnippet returns every SELECTABLE prompt node whose Q
// text contains the collapsed snippet. Skipped prompts are not
// selectable: production drops them after the overlay, so selecting one
// would leave the fixture walking nothing and passing vacuously.
func findPromptsBySnippet(sections *yaml.Node, snippet string) []*yaml.Node {
	hits := make([]*yaml.Node, 0)

	for _, section := range sections.Content {
		prompts := mappingValue(section, "validation_prompts")
		if prompts == nil {
			continue
		}

		for _, prompt := range prompts.Content {
			if promptSkipped(prompt) {
				continue
			}

			question := mappingValue(prompt, "Q")
			if question == nil {
				continue
			}

			if strings.Contains(collapseSpaces(question.Value), snippet) {
				hits = append(hits, prompt)
			}
		}
	}

	return hits
}

// promptSkipped mirrors the production skip rule exactly: a prompt is
// skipped only when its `skip` value is a NON-EMPTY STRING — booleans
// and other types decode to "" in the production map-based decoder and
// do not skip.
func promptSkipped(prompt *yaml.Node) bool {
	skip := mappingValue(prompt, "skip")
	if skip == nil || skip.Kind != yaml.ScalarNode {
		return false
	}

	return skip.ShortTag() == "!!str" && strings.TrimSpace(skip.Value) != ""
}

// parseSingleDocument parses checklist YAML, rejecting multi-document
// streams (only the first document would survive the round trip).
func parseSingleDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	var doc yaml.Node

	err := decoder.Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("parsing checklist: %w", err)
	}

	var extra yaml.Node

	err = decoder.Decode(&extra)
	if err == nil {
		return nil, fmt.Errorf("%w: multiple documents", ErrUnsupportedYAML)
	}

	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing trailing document: %w", err)
	}

	return &doc, nil
}

// rejectAliases refuses documents containing alias nodes or merge keys:
// pruning prompts could orphan an anchor and emit an unparsable file.
func rejectAliases(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("%w: alias node", ErrUnsupportedYAML)
	}

	if node.Kind == yaml.MappingNode {
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			if node.Content[idx].Value == "<<" {
				return fmt.Errorf("%w: merge key", ErrUnsupportedYAML)
			}
		}
	}

	for _, child := range node.Content {
		err := rejectAliases(child)
		if err != nil {
			return err
		}
	}

	return nil
}

// documentRoot unwraps the yaml document node.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}

	return nil
}

// mappingValue returns the value node for a key of a mapping node.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx+1]
		}
	}

	return nil
}

// collapseSpaces joins all whitespace runs to single spaces, so
// snippets match across YAML block-scalar line reflow.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
