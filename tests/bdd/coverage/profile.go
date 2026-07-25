package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// sortObservations orders observations deterministically so profiles
// are byte-reproducible across runs.
func sortObservations(observations []Observation) {
	sort.Slice(observations, func(i, j int) bool {
		left, right := observations[i], observations[j]

		leftKey := left.Session + "\x00" + left.Fixture + "\x00" + left.Partition + "\x00" +
			left.Subject + "\x00" + string(left.Kind) + "\x00" + left.Evidence
		rightKey := right.Session + "\x00" + right.Fixture + "\x00" + right.Partition + "\x00" +
			right.Subject + "\x00" + string(right.Kind) + "\x00" + right.Evidence

		if left.PromptIndex != right.PromptIndex {
			return left.PromptIndex < right.PromptIndex
		}

		return leftKey < rightKey
	})
}

// profileSchema versions the wire format.
const profileSchema = "true-bdd.checklist-coverage-profile/bootstrap-v1"

// Branch states in profiles and reports.
const (
	stateCovered   = "covered"
	stateUncovered = "uncovered"
)

// filePerm is the mode for files this tool writes.
const filePerm = 0o644

// evidenceQualityReconstructed labels every bootstrap profile: derived
// from retained artifacts, without ratchet authority.
const evidenceQualityReconstructed = "reconstructed"

// ProfileBranch is the merged state of one universe branch.
type ProfileBranch struct {
	ID       string   `json:"id"`
	QSHA256  string   `json:"q_sha256"`
	FSHA256  string   `json:"f_sha256,omitempty"`
	State    string   `json:"state"` // "covered" | "uncovered"
	Count    int      `json:"count"`
	Fixtures []string `json:"fixtures,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	// EvidenceOmitted counts observations beyond the evidence cap, so
	// count and evidence length always reconcile.
	EvidenceOmitted int `json:"evidence_omitted,omitempty"`
}

// ProfileDiagnostics keeps the uncredited signals visible.
type ProfileDiagnostics struct {
	ProtocolFails      []string `json:"protocol_fails,omitempty"`
	NonCanonical       []string `json:"non_canonical,omitempty"`
	NonShipped         []string `json:"non_shipped,omitempty"`
	FixCandidates      []string `json:"fix_candidates,omitempty"`
	UnattributedApply  []string `json:"unattributed_applies,omitempty"`
	FailUnclassified   []string `json:"fail_unclassified,omitempty"`
	RunDiagnostics     []string `json:"run_diagnostics,omitempty"`
	HardRunDiagnostics []string `json:"hard_run_diagnostics,omitempty"`
}

// Profile is the merged coverage state over every scanned run.
type Profile struct {
	Schema          string             `json:"schema"`
	UniverseSHA256  string             `json:"universe_sha256"`
	EvidenceQuality string             `json:"evidence_quality"`
	Sessions        []string           `json:"sessions"`
	Branches        []ProfileBranch    `json:"branches"`
	Diagnostics     ProfileDiagnostics `json:"diagnostics"`
}

// evidenceCap bounds per-branch evidence lists in the profile.
const evidenceCap = 4

// BuildProfile merges observations into per-branch coverage state over
// the full universe.
func BuildProfile(uni *Universe, observations []Observation, diagnostics []Diagnostic) *Profile {
	merge := newMergeState(uni)

	sortObservations(observations)

	for _, obs := range observations {
		merge.add(obs)
	}

	profile := &Profile{
		Schema:          profileSchema,
		UniverseSHA256:  uni.SHA256(),
		EvidenceQuality: evidenceQualityReconstructed,
		Sessions:        merge.sessions(),
		Branches:        merge.branches(),
		Diagnostics:     merge.diagnostics,
	}

	for _, diag := range diagnostics {
		line := diag.Fixture + ": " + diag.Message

		if diag.Hard {
			profile.Diagnostics.HardRunDiagnostics =
				append(profile.Diagnostics.HardRunDiagnostics, line)
		} else {
			profile.Diagnostics.RunDiagnostics =
				append(profile.Diagnostics.RunDiagnostics, line)
		}
	}

	profile.Diagnostics.sortAll()

	return profile
}

// sortAll orders every diagnostic bucket so profiles are reproducible
// even when diagnostics originate from map-ordered cell iteration.
func (d *ProfileDiagnostics) sortAll() {
	for _, bucket := range [][]string{
		d.ProtocolFails, d.NonCanonical, d.NonShipped, d.FixCandidates,
		d.UnattributedApply, d.FailUnclassified, d.RunDiagnostics, d.HardRunDiagnostics,
	} {
		sort.Strings(bucket)
	}
}

// WriteJSON serializes the profile.
func (p *Profile) WriteJSON(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}

	err = os.WriteFile(path, append(data, '\n'), filePerm)
	if err != nil {
		return fmt.Errorf("writing profile %s: %w", path, err)
	}

	return nil
}

// CoveredIDs returns the sorted ids of covered branches.
func (p *Profile) CoveredIDs() []string {
	ids := make([]string, 0)

	for _, b := range p.Branches {
		if b.State == stateCovered {
			ids = append(ids, b.ID)
		}
	}

	sort.Strings(ids)

	return ids
}

// branchAccumulator gathers evidence for one branch during merging.
type branchAccumulator struct {
	count    int
	fixtures map[string]bool
	evidence []string
	omitted  int
	dedup    map[string]bool
}

// mergeState is the working state of BuildProfile.
type mergeState struct {
	uni         *Universe
	perBranch   map[string]*branchAccumulator // key: branch machine key
	sessionSet  map[string]bool
	diagnostics ProfileDiagnostics
}

// newMergeState wires the accumulator maps.
func newMergeState(uni *Universe) *mergeState {
	return &mergeState{
		uni:        uni,
		perBranch:  map[string]*branchAccumulator{},
		sessionSet: map[string]bool{},
	}
}

// branchKeyFor is the machine matching key: checklist plus hashes plus
// kind, never the display locator. The checklist name prevents an
// identical Q text in another checklist from cross-crediting.
func branchKeyFor(checklist, qHash, fHash string, kind BranchKind) string {
	return checklist + "/" + qHash + "+" + fHash + "#" + string(kind)
}

// add folds one observation into the merge state.
func (m *mergeState) add(obs Observation) {
	m.sessionSet[obs.Session] = true

	kind, credits := creditKind(obs.Kind)
	if !credits {
		m.addDiagnostic(obs)

		return
	}

	if obs.Prompt == nil {
		m.diagnostics.NonShipped = append(m.diagnostics.NonShipped, evidenceLine(obs))

		return
	}

	key := branchKeyFor(obs.Prompt.Checklist, obs.Prompt.QHash, obs.Prompt.FHash, kind)

	acc := m.perBranch[key]
	if acc == nil {
		acc = &branchAccumulator{fixtures: map[string]bool{}, dedup: map[string]bool{}}
		m.perBranch[key] = acc
	}

	dedupKey := fmt.Sprintf("%s/%s/%s/%s/%s/%d",
		obs.Session, obs.Fixture, obs.Partition, obs.Subject, obs.Kind, obs.PromptIndex)
	if acc.dedup[dedupKey] {
		return
	}

	acc.dedup[dedupKey] = true
	acc.count++
	acc.fixtures[obs.Fixture] = true

	if len(acc.evidence) < evidenceCap {
		acc.evidence = append(acc.evidence, obs.Evidence)
	} else {
		acc.omitted++
	}
}

// creditKind maps observation kinds to the branch kind they credit.
func creditKind(kind ObservationKind) (BranchKind, bool) {
	switch kind {
	case ObsPass:
		return KindPass, true
	case ObsFailSemantic:
		return KindFail, true
	case ObsFixEffective:
		return KindFix, true
	case ObsFailProtocol, ObsNonCanonical, ObsFixCandidate,
		ObsNonShipped, ObsUnattributed, ObsFailUnclassed:
		return "", false
	default:
		return "", false
	}
}

// addDiagnostic files a non-crediting observation into its bucket.
func (m *mergeState) addDiagnostic(obs Observation) {
	line := evidenceLine(obs)

	switch obs.Kind {
	case ObsFailProtocol:
		m.diagnostics.ProtocolFails = append(m.diagnostics.ProtocolFails, line)
	case ObsNonCanonical:
		m.diagnostics.NonCanonical = append(m.diagnostics.NonCanonical, line)
	case ObsNonShipped:
		m.diagnostics.NonShipped = append(m.diagnostics.NonShipped, line)
	case ObsFixCandidate:
		m.diagnostics.FixCandidates = append(m.diagnostics.FixCandidates, line)
	case ObsUnattributed:
		m.diagnostics.UnattributedApply = append(m.diagnostics.UnattributedApply, line)
	case ObsFailUnclassed:
		m.diagnostics.FailUnclassified = append(m.diagnostics.FailUnclassified, line)
	case ObsPass, ObsFailSemantic, ObsFixEffective:
	}
}

// evidenceLine renders one diagnostic evidence line.
func evidenceLine(obs Observation) string {
	return fmt.Sprintf("%s [%s] %s (%s)", obs.Fixture, obs.Kind, obs.Evidence, obs.Note)
}

// sessions returns the sorted distinct sessions.
func (m *mergeState) sessions() []string {
	sessions := make([]string, 0, len(m.sessionSet))
	for s := range m.sessionSet {
		sessions = append(sessions, s)
	}

	sort.Strings(sessions)

	return sessions
}

// branches renders the full universe with merged state.
func (m *mergeState) branches() []ProfileBranch {
	branches := make([]ProfileBranch, 0, len(m.uni.Branches))

	for _, b := range m.uni.Branches {
		profBranch := ProfileBranch{
			ID: b.ID(), QSHA256: b.QHash, FSHA256: b.FHash, State: stateUncovered,
		}

		if acc := m.perBranch[branchKeyFor(b.Checklist, b.QHash, b.FHash, b.Kind)]; acc != nil {
			profBranch.State = stateCovered
			profBranch.Count = acc.count
			profBranch.Fixtures = sortedKeys(acc.fixtures)
			profBranch.Evidence = acc.evidence
			profBranch.EvidenceOmitted = acc.omitted
		}

		branches = append(branches, profBranch)
	}

	return branches
}

// sortedKeys returns the sorted keys of a string set.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
