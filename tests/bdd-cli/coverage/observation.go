package main

import "errors"

// ObservationKind labels what one piece of evidence proves.
type ObservationKind string

// Observation kinds. Only the first three grant branch credit; the
// rest are diagnostics kept visible in the report.
const (
	ObsPass          ObservationKind = "pass"
	ObsFailSemantic  ObservationKind = "fail_semantic"
	ObsFixEffective  ObservationKind = "fix_effective"
	ObsFailProtocol  ObservationKind = "fail_protocol"
	ObsNonCanonical  ObservationKind = "non_canonical"
	ObsFixCandidate  ObservationKind = "fix_candidate"
	ObsNonShipped    ObservationKind = "non_shipped"
	ObsUnattributed  ObservationKind = "apply_unattributed"
	ObsFailUnclassed ObservationKind = "fail_unclassified"
)

// Observation is one dedup-keyed piece of evidence extracted from a
// fixture run: (session, partition, subject, prompt, kind).
type Observation struct {
	Fixture     string
	Session     string
	Partition   string // partition basename
	Subject     string
	PromptIndex int // run-local flattened index, 0 when unknown
	Kind        ObservationKind
	Prompt      *UniversePrompt // shipped prompt earning credit, nil otherwise
	Evidence    string          // path of the primary evidence file
	Note        string
}

// Diagnostic is a run-level defect that must stay visible and, when
// hard, blocks baseline writing.
type Diagnostic struct {
	Fixture string
	Hard    bool
	Message string
}

// errCommandConflict signals disagreeing command evidence between the
// fixture manifest and the log.
var errCommandConflict = errors.New("conflicting command evidence")
