package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// BranchKind is the direction of a checklist-content branch.
type BranchKind string

// The three checklist-content branch kinds of the verified model:
// each Q has a pass and a fail branch; each authored F has one
// composite fix-effective branch.
const (
	KindPass BranchKind = "pass"
	KindFail BranchKind = "fail"
	KindFix  BranchKind = "fix"
)

// Branch is one checklist-content branch of the coverage universe.
type Branch struct {
	Checklist string
	SectionID string
	Ordinal   int // 1-based ordinal within the section
	Global    int // 1-based index over the flattened prompt list
	Kind      BranchKind
	QHash     string // full sha256 hex over the collapsed Q text
	FHash     string // full sha256 hex over the collapsed F text, "" when no F
	Snippet   string // first characters of the collapsed Q, for humans
}

// ID renders the display identity, e.g. "us-create/format/q1@75797b9f#pass".
// The locator part (section/ordinal) is display-only; machine matching
// uses the full hashes.
func (b Branch) ID() string {
	return fmt.Sprintf("%s/%s/q%d@%s#%s",
		b.Checklist, b.SectionID, b.Ordinal, shortHash(b.QHash), b.Kind)
}

// collapseWhitespace joins all whitespace runs to single spaces so YAML
// block-scalar reflow does not churn identity, while any word change does.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// hashText returns the full sha256 hex of the collapsed text.
func hashText(s string) string {
	sum := sha256.Sum256([]byte(collapseWhitespace(s)))

	return hex.EncodeToString(sum[:])
}

// shortHash returns the 8-hex display form of a full hash.
func shortHash(full string) string {
	const displayLen = 8

	if len(full) < displayLen {
		return full
	}

	return full[:displayLen]
}

// snippet returns the first n characters of the collapsed text.
func snippet(s string, maxLen int) string {
	collapsed := collapseWhitespace(s)
	if len(collapsed) <= maxLen {
		return collapsed
	}

	return collapsed[:maxLen] + "…"
}
