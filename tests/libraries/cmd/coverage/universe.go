package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UniversePrompt is one flattened, non-skipped shipped validation prompt
// together with everything needed for eval-field matching.
type UniversePrompt struct {
	Checklist     string
	SectionID     string
	Ordinal       int
	Global        int
	QHash         string
	FHash         string
	QCollapsed    string
	EvalKey       string // digest over all evaluator-relevant fields
	HasF          bool
	Snippet       string
	SanitizedPath string // sanitizeID("<checklist>/<sectionID>") as used in artifact names
}

// Universe is the shipped-checklist branch universe (59 branches today).
type Universe struct {
	Prompts  []UniversePrompt
	Branches []Branch
	// byEvalKey maps "<checklist>\x00<evalKey>" to the prompt, for
	// granting shipped credit to override prompts that match exactly.
	byEvalKey map[string]*UniversePrompt
	// Diagnostics collects universe-level defects (duplicate hashes).
	Diagnostics []string
}

// LoadUniverse reads every *.yaml under checklistsDir and builds the
// branch universe with loader-parity flattening.
func LoadUniverse(checklistsDir string) (*Universe, error) {
	paths, err := filepath.Glob(filepath.Join(checklistsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing checklists in %s: %w", checklistsDir, err)
	}

	sort.Strings(paths)

	uni := &Universe{byEvalKey: map[string]*UniversePrompt{}}

	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".yaml")

		prompts, loadErr := loadChecklistPrompts(path, stem)
		if loadErr != nil {
			return nil, loadErr
		}

		uni.addPrompts(stem, prompts)
	}

	uni.buildBranches()

	return uni, nil
}

// MatchOverride returns the shipped prompt that is evaluator-equivalent
// to the given loaded prompt, or nil when the prompt is non-shipped.
func (u *Universe) MatchOverride(checklist string, prompt UniversePrompt) *UniversePrompt {
	return u.byEvalKey[checklist+"\x00"+prompt.EvalKey]
}

// SHA256 digests the sorted branch ids plus hashes, identifying the
// universe revision a profile or baseline was computed against.
func (u *Universe) SHA256() string {
	ids := make([]string, 0, len(u.Branches))
	for _, b := range u.Branches {
		ids = append(ids, b.ID()+"@"+b.QHash+"+"+b.FHash)
	}

	sort.Strings(ids)

	sum := sha256.Sum256([]byte(strings.Join(ids, "\n")))

	return hex.EncodeToString(sum[:])
}

// addPrompts registers one checklist's flattened prompts and records
// duplicate-identity diagnostics.
func (u *Universe) addPrompts(stem string, prompts []UniversePrompt) {
	seen := map[string]int{}

	for i := range prompts {
		prompt := prompts[i]

		if prev, dup := seen[prompt.QHash]; dup {
			u.Diagnostics = append(u.Diagnostics, fmt.Sprintf(
				"duplicate Q hash within %s: prompts %d and %d", stem, prev, prompt.Global))
		}

		seen[prompt.QHash] = prompt.Global

		u.Prompts = append(u.Prompts, prompt)
		u.byEvalKey[stem+"\x00"+prompt.EvalKey] = &u.Prompts[len(u.Prompts)-1]
	}
}

// buildBranches derives the pass/fail(/fix) branches from the prompts.
func (u *Universe) buildBranches() {
	for _, prompt := range u.Prompts {
		base := Branch{
			Checklist: prompt.Checklist,
			SectionID: prompt.SectionID,
			Ordinal:   prompt.Ordinal,
			Global:    prompt.Global,
			QHash:     prompt.QHash,
			FHash:     prompt.FHash,
			Snippet:   prompt.Snippet,
		}

		for _, kind := range []BranchKind{KindPass, KindFail} {
			b := base
			b.Kind = kind
			u.Branches = append(u.Branches, b)
		}

		if prompt.HasF {
			b := base
			b.Kind = KindFix
			u.Branches = append(u.Branches, b)
		}
	}
}

// loadChecklistPrompts parses one checklist file and flattens it with
// production parity (skip = any non-empty string, order preserved,
// 1-based global index over the filtered list).
func loadChecklistPrompts(path, stem string) ([]UniversePrompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading checklist %s: %w", path, err)
	}

	doc, err := parseChecklistDoc(data)
	if err != nil {
		return nil, fmt.Errorf("parsing checklist %s: %w", path, err)
	}

	prompts := make([]UniversePrompt, 0)
	global := 0

	for _, section := range doc.Sections {
		ordinal := 0

		for _, prompt := range section.ValidationPrompts {
			if prompt.Skip != "" {
				continue
			}

			global++
			ordinal++

			prompts = append(prompts,
				newUniversePrompt(stem, section.ID, ordinal, global, prompt, doc.DefaultDocs))
		}
	}

	return prompts, nil
}

const snippetLen = 46

// newUniversePrompt computes the hashes and eval key for one prompt.
func newUniversePrompt(
	stem, sectionID string,
	ordinal, global int,
	doc promptDoc,
	defaultDocs []string,
) UniversePrompt {
	docs := doc.Docs
	if len(docs) == 0 {
		docs = defaultDocs
	}

	evalKey := evalFieldsDigest(doc.Question, doc.Rationale, doc.FixTemplate, docs)

	return UniversePrompt{
		Checklist:     stem,
		SectionID:     sectionID,
		Ordinal:       ordinal,
		Global:        global,
		QHash:         hashText(doc.Question),
		FHash:         fHashOf(doc.FixTemplate),
		QCollapsed:    collapseWhitespace(doc.Question),
		EvalKey:       evalKey,
		HasF:          strings.TrimSpace(doc.FixTemplate) != "",
		Snippet:       snippet(doc.Question, snippetLen),
		SanitizedPath: sanitizeID(stem + "/" + sectionID),
	}
}

// fHashOf hashes a fix template, mapping an absent template to "".
func fHashOf(f string) string {
	if strings.TrimSpace(f) == "" {
		return ""
	}

	return hashText(f)
}

// evalFieldsDigest digests every evaluator-relevant prompt field: Q,
// rationale, F, and the effective docs list. Docs are length-prefixed
// so ["a","b"] cannot collide with ["a,b"] (different production input).
func evalFieldsDigest(question, rationale, fixTemplate string, docs []string) string {
	docParts := make([]string, 0, len(docs))
	for _, doc := range docs {
		docParts = append(docParts, fmt.Sprintf("%d:%s", len(doc), doc))
	}

	parts := []string{
		collapseWhitespace(question),
		collapseWhitespace(rationale),
		collapseWhitespace(fixTemplate),
		strings.Join(docParts, ","),
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))

	return hex.EncodeToString(sum[:])
}
