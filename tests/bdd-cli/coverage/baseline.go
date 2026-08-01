package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// baselineSchemaVersion versions the baseline wire format.
const baselineSchemaVersion = 1

// BaselineEntry pins one covered branch by machine hashes; the id is
// the display form current at write time.
type BaselineEntry struct {
	ID      string `yaml:"id"`
	QSHA256 string `yaml:"q_sha256"`
	FSHA256 string `yaml:"f_sha256,omitempty"`
}

// Baseline is the checked-in covered set. A bootstrap baseline is
// always evidence_quality: reconstructed — it has no ratchet authority
// until a fresh instrumented suite reproduces it.
type Baseline struct {
	SchemaVersion   int             `yaml:"schema_version"`
	EvidenceQuality string          `yaml:"evidence_quality"`
	UniverseSHA256  string          `yaml:"universe_sha256"`
	Covered         []BaselineEntry `yaml:"covered"`
}

// errBaselineRegression signals covered branches that regressed.
var errBaselineRegression = errors.New("baseline regression")

// errHardDiagnostics blocks baseline writing while hard defects exist.
var errHardDiagnostics = errors.New("hard diagnostics present; refusing to write baseline")

// errBaselineShrink blocks a write that would drop existing entries.
var errBaselineShrink = errors.New(
	"write would remove existing baseline entries; rerun with -rebaseline to allow")

// errBaselineInvalid signals a malformed baseline file.
var errBaselineInvalid = errors.New("invalid baseline")

// sha256HexRe matches a full lowercase sha256 hex digest.
var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// BaselineFromProfile derives a baseline from the merged profile.
func BaselineFromProfile(profile *Profile) *Baseline {
	baseline := &Baseline{
		SchemaVersion:   baselineSchemaVersion,
		EvidenceQuality: profile.EvidenceQuality,
		UniverseSHA256:  profile.UniverseSHA256,
	}

	for _, b := range profile.Branches {
		if b.State == stateCovered {
			baseline.Covered = append(baseline.Covered,
				BaselineEntry{ID: b.ID, QSHA256: b.QSHA256, FSHA256: b.FSHA256})
		}
	}

	sort.Slice(baseline.Covered, func(i, j int) bool {
		return baseline.Covered[i].ID < baseline.Covered[j].ID
	})

	return baseline
}

// Write persists the baseline atomically (write + rename). It is
// fail-closed: hard diagnostics block it, and an existing baseline may
// only shrink when allowShrink is set explicitly.
func (b *Baseline) Write(path string, hardDiagnostics []string, allowShrink bool) error {
	if len(hardDiagnostics) > 0 {
		return fmt.Errorf("%w: %d issue(s), first: %s",
			errHardDiagnostics, len(hardDiagnostics), hardDiagnostics[0])
	}

	err := b.guardShrink(path, allowShrink)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshaling baseline: %w", err)
	}

	tmpPath := path + ".tmp"

	err = os.WriteFile(tmpPath, data, filePerm)
	if err != nil {
		return fmt.Errorf("writing baseline temp file: %w", err)
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		return fmt.Errorf("renaming baseline into place: %w", err)
	}

	return nil
}

// LoadBaseline reads a baseline file.
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading baseline %s: %w", path, err)
	}

	var baseline Baseline

	err = yaml.Unmarshal(data, &baseline)
	if err != nil {
		return nil, fmt.Errorf("parsing baseline %s: %w", path, err)
	}

	err = baseline.validate(path)
	if err != nil {
		return nil, err
	}

	return &baseline, nil
}

// Check compares the baseline against a profile: every baselined
// branch (matched by hashes, reorder-safe) must still be covered.
// It returns the newly covered ids as candidates for a ratchet update.
func (b *Baseline) Check(profile *Profile) ([]string, error) {
	coveredByKey := map[string]bool{}
	currentIDs := map[string]bool{}

	for _, pb := range profile.Branches {
		key := entryKey(pb.ID, pb.QSHA256, pb.FSHA256)
		currentIDs[pb.ID] = true

		if pb.State == stateCovered {
			coveredByKey[key] = true
		}
	}

	missing := make([]string, 0)

	for _, entry := range b.Covered {
		if !coveredByKey[entryKey(entry.ID, entry.QSHA256, entry.FSHA256)] {
			missing = append(missing, entry.ID)
		}
	}

	newlyCovered := b.newlyCovered(profile)

	if len(missing) > 0 {
		return newlyCovered, fmt.Errorf("%w: %d baselined branch(es) not covered: %v",
			errBaselineRegression, len(missing), missing)
	}

	return newlyCovered, nil
}

// newlyCovered lists covered branches absent from the baseline.
func (b *Baseline) newlyCovered(profile *Profile) []string {
	baselined := map[string]bool{}
	for _, entry := range b.Covered {
		baselined[entryKey(entry.ID, entry.QSHA256, entry.FSHA256)] = true
	}

	ids := make([]string, 0)

	for _, pb := range profile.Branches {
		if pb.State == stateCovered && !baselined[entryKey(pb.ID, pb.QSHA256, pb.FSHA256)] {
			ids = append(ids, pb.ID)
		}
	}

	sort.Strings(ids)

	return ids
}

// guardShrink refuses to drop entries an existing baseline holds. Only
// a genuinely absent file means "nothing to preserve" — an unreadable
// or invalid existing baseline aborts the write instead of being
// silently replaced (unless the shrink override is explicit).
func (b *Baseline) guardShrink(path string, allowShrink bool) error {
	existing, err := LoadBaseline(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // nothing to preserve
		}

		if allowShrink {
			return nil // explicit rebaseline may replace a broken file
		}

		return fmt.Errorf("existing baseline is unreadable or invalid; "+
			"refusing to overwrite without -rebaseline: %w", err)
	}

	newKeys := map[string]bool{}
	for _, entry := range b.Covered {
		newKeys[entryKey(entry.ID, entry.QSHA256, entry.FSHA256)] = true
	}

	dropped := make([]string, 0)

	for _, entry := range existing.Covered {
		if !newKeys[entryKey(entry.ID, entry.QSHA256, entry.FSHA256)] {
			dropped = append(dropped, entry.ID)
		}
	}

	if len(dropped) > 0 && !allowShrink {
		return fmt.Errorf("%w: %d entries would be dropped: %v",
			errBaselineShrink, len(dropped), dropped)
	}

	return nil
}

// validate rejects malformed baselines instead of silently checking
// against them.
func (b *Baseline) validate(path string) error {
	if b.SchemaVersion != baselineSchemaVersion {
		return fmt.Errorf("%w %s: schema_version %d, want %d",
			errBaselineInvalid, path, b.SchemaVersion, baselineSchemaVersion)
	}

	if b.EvidenceQuality != evidenceQualityReconstructed {
		return fmt.Errorf("%w %s: evidence_quality %q, want %q (the bootstrap tool "+
			"only produces reconstructed baselines)",
			errBaselineInvalid, path, b.EvidenceQuality, evidenceQualityReconstructed)
	}

	if !sha256HexRe.MatchString(b.UniverseSHA256) {
		return fmt.Errorf("%w %s: universe_sha256 is not a sha256 hex digest",
			errBaselineInvalid, path)
	}

	seen := map[string]bool{}

	for _, entry := range b.Covered {
		key := entryKey(entry.ID, entry.QSHA256, entry.FSHA256)
		if seen[key] {
			return fmt.Errorf("%w %s: duplicate entry %s", errBaselineInvalid, path, entry.ID)
		}

		seen[key] = true
	}

	return nil
}

// entryKey builds the reorder-safe machine matching key of one entry:
// checklist (from the id prefix) plus hashes plus kind. Section and
// ordinal are deliberately excluded.
func entryKey(id, qSHA, fSHA string) string {
	return checklistOfID(id) + "/" + qSHA + "+" + fSHA + "#" + kindOfID(id)
}

// checklistOfID extracts the checklist prefix of a display id.
func checklistOfID(id string) string {
	for i := range len(id) {
		if id[i] == '/' {
			return id[:i]
		}
	}

	return ""
}

// kindOfID extracts the branch kind suffix of a display id.
func kindOfID(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '#' {
			return id[i+1:]
		}
	}

	return ""
}
