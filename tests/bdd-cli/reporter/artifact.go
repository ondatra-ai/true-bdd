package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Records naming an artifact the engine wrote.
const (
	msgPromptSaved     = "Prompt saved"
	msgResultSaved     = "Result file saved"
	msgFixPromptSaved  = "Fix prompt saved"
	msgArchitecture    = "Loaded architecture"
	msgChecklistLoad   = "Loading full checklist"
	msgPromptsLoaded   = "Loaded prompts"
	msgFixGenerating   = "Generating fix prompt"
	msgFixChoice       = "User chose fix action"
	msgFixApplying     = "Applying fix prompt"
	msgFixNotApplied   = "Fix was NOT applied"
	msgToolsConfigured = "Claude tools configured"
	msgToolUse         = "ToolUseBlock details"
	msgSpawnAgent      = "Spawning agent CLI"
	msgSpawnRunner     = "Spawning test runner"
	msgRunnerReturned  = "Test runner returned"
)

// Artifact is one file the engine wrote during a turn, with its content.
// Prompts and responses are read eagerly: they are the evidence the
// detail page exists to show, and a fixture's whole artifact set is tens
// of kilobytes.
type Artifact struct {
	Kind    string
	Name    string
	Path    string
	Content string
	Bytes   int
	Missing bool
}

// artifactKind labels an artifact from its filename suffix, so the
// detail page can order inputs before outputs rather than listing files.
func artifactKind(name string) string {
	switch {
	case strings.HasSuffix(name, "-system.txt"):
		return "system prompt"
	case strings.HasSuffix(name, "-user.txt"):
		return "user prompt"
	case strings.HasSuffix(name, "-response.txt"):
		return "response"
	case strings.HasSuffix(name, "-result.yaml"):
		return "result"
	case strings.HasSuffix(name, "-fix-prompts.md"):
		return "fix prompt"
	case strings.HasSuffix(name, "-crush.log"), strings.HasSuffix(name, "-codex.log"):
		return "CLI transcript"
	default:
		return "artifact"
	}
}

// loadArtifact reads one artifact by the path the engine logged. A file
// that has since been deleted is reported as missing rather than
// silently dropped — the run did write it.
func loadArtifact(fixtureDir, logged string) Artifact {
	path := logged
	if !filepath.IsAbs(path) {
		path = filepath.Join(fixtureDir, logged)
	}

	artifact := Artifact{
		Kind: artifactKind(logged),
		Name: filepath.Base(logged),
		Path: logged,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		artifact.Missing = true

		return artifact
	}

	artifact.Content = string(content)
	artifact.Bytes = len(content)

	return artifact
}

// isArtifactRecord reports whether a record names a file the engine
// wrote.
func isArtifactRecord(msg string) bool {
	switch msg {
	case msgPromptSaved, msgResultSaved, msgFixPromptSaved, msgTranscriptSaved:
		return true
	default:
		return false
	}
}

// checklistCell identifies which checklist cell produced a turn.
//
// Derived from the artifact filename, which the engine builds as
// `<promptIndex>-<subjectID>-checklist-<section-path>-<suffix>.txt`
// (checklist_evaluator.go: savePromptFile). The fix and apply turns of
// the same cell reuse the subject and index, so the whole three-turn
// cycle resolves to one cell.
type checklistCell struct {
	Index   string
	Subject string
	Section string
}

// String renders the cell for a summary line.
func (c checklistCell) String() string {
	if c.Section == "" && c.Subject == "" {
		return ""
	}

	if c.Section == "" {
		return c.Subject
	}

	return c.Section + " · " + c.Subject
}

// iterationSuffix matches the per-iteration markers the fix generator
// and the applier append (`-fix-iter1`, `-iter2`).
var iterationSuffix = regexp.MustCompile(`-(fix-)?iter\d+$`)

// cellFromArtifact parses a cell out of an artifact filename. Returns
// false for names that carry no cell (the crush transcript, say).
//
// Three shapes exist, all built from the same subject id:
//
//	01-<subject>-checklist-<section>-user.txt   the validation turn
//	01-<subject>-fix-iter1-user.txt             the fix turn
//	apply-<subject>-iter1-user.txt              the apply turn
//
// Only the first names the section, so the other two resolve to the same
// subject and inherit the section from it (see inheritCellSections).
func cellFromArtifact(name string) (checklistCell, bool) {
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".txt"), ".yaml")

	index, rest, found := strings.Cut(base, "-")
	if !found {
		return checklistCell{}, false
	}

	if index == "apply" {
		subject := trimArtifactSuffix(rest)

		return checklistCell{Subject: subject}, subject != ""
	}

	if !isNumeric(index) {
		return checklistCell{}, false
	}

	cell := checklistCell{Index: index}

	subject, section, hasSection := strings.Cut(rest, "-checklist-")
	if hasSection {
		cell.Subject = strings.Trim(subject, "-")
		cell.Section = trimArtifactSuffix(section)

		return cell, true
	}

	cell.Subject = trimArtifactSuffix(rest)

	return cell, cell.Subject != ""
}

// trimArtifactSuffix strips the role and iteration markers the engine
// appends, leaving the bare subject or section.
func trimArtifactSuffix(name string) string {
	for _, suffix := range []string{
		"-system", "-user", "-response", "-result", "-fix-prompts",
	} {
		name = strings.TrimSuffix(name, suffix)
	}

	name = iterationSuffix.ReplaceAllString(name, "")

	return strings.Trim(name, "-")
}

// inheritCellSections fills in the section for turns whose artifacts do
// not name one. The fix and apply turns of a cell reuse its subject, so
// the validation turn that opened the cell supplies the section for the
// whole cycle — which is what makes a turn readable as "this cell",
// rather than as three unrelated subjects.
func inheritCellSections(turns []*Turn) {
	sections := map[string]string{}

	for _, turn := range turns {
		if turn.Cell.Subject != "" && turn.Cell.Section != "" {
			sections[turn.Cell.Subject] = turn.Cell.Section
		}
	}

	for _, turn := range turns {
		if turn.Cell.Section != "" || turn.Cell.Subject == "" {
			continue
		}

		if section, ok := sections[turn.Cell.Subject]; ok {
			turn.Cell.Section = section
		}
	}
}

// isNumeric reports whether every rune is a digit.
func isNumeric(text string) bool {
	if text == "" {
		return false
	}

	for _, char := range text {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}
