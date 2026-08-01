// Package inventory is the host-folder scanner with a complete identity
// model (plan §3.4). It mirrors the engine loader semantics (epic,
// story, registry, checklist, config) without constructing a bootstrap
// container, so it runs honestly in a bare or degraded folder and never
// panics on malformed input. The remote uploads the resulting Snapshot;
// the harness server stores it opaquely and the browser renders it (the
// data-testid contract lives in tests/harness/helpers/README-testids.md).
//
// # Snapshot JSON schema (binding for the server + UI phases)
//
// documents is a key→status map (the UI renders each on an
// inventory-doc-<key> chip's data-status attribute); document_errors
// carries the parse error for the `invalid` chips only.
//
//	{
//	  "documents": {                       // key -> status
//	    "config":          "present|missing|invalid|present_empty",
//	    "prd":             "present|missing|invalid",
//	    "architecture":    "present|missing|invalid",
//	    "registry":        "present|missing|invalid|present_empty",
//	    "stories-dir":     "present|missing|not_a_dir",
//	    "epics-dir":       "present|missing|not_a_dir",
//	    "checklist-us-create": "present|missing|invalid",
//	    "checklist-us-refine": "...", "checklist-us-apply": "...",
//	    "checklist-build-tests": "...", "checklist-build-code": "..."
//	  },
//	  "document_errors": {"config": "yaml: line 3: ..."},  // invalid chips only
//	  "architecture_path_mismatch": bool,  // configured != canonical default
//	  "configured_architecture_path": "docs/arch/architecture.yaml",
//	  "canonical_architecture_path":  "docs/architecture/architecture.yaml",
//	  "epics": [
//	    {
//	      "file": "epic-42-identity-mismatch.yaml",   // basename
//	      "number": 42,                               // filename number, no leading zeros
//	      "status": "parseable|invalid",
//	      "doc_id": 99,                               // document epic.id (0 when unparseable)
//	      "id_mismatch": true,                        // doc_id present and != number
//	      "duplicate_number": false,                  // shares a filename number with another epic
//	      "noncanonical_filename": false,             // filename number is not the %02d form us create resolves;
//	                                                  // when true the epic carries no Create-addressable stories
//	      "error": "?",
//	      "stories": [
//	        {
//	          "create_id": "42.1",       // position-derived <number>.<position>
//	          "epic_number": 42,
//	          "position": 1,
//	          "declared_id": "77.5",     // epic-declared story id
//	          "file_id": "88.9",         // story-file internal id, when resolvable
//	          "created": "one|missing|ambiguous|invalid",
//	          "applied": {"status": "counted", "applied": 1, "total": 2}
//	                   | {"status": "unknown", "reason":
//	                        "missing|ambiguous|invalid|deprecated_format|no_acceptance_criteria|
//	                         empty_internal_id|registry_missing|registry_invalid"},
//	          "refined": "not_recorded",
//	          "flags": {"duplicate_declared_id": bool, "id_mismatch": bool,
//	                    "deprecated_format": bool, "no_acs": bool, "empty_internal_id": bool}
//	        }
//	      ]
//	    }
//	  ]
//	}
package inventory

// Document / directory / checklist chip statuses (plan §3.4). The UI
// renders each on an inventory-doc-<key> chip's data-status attribute.
const (
	// StatusPresent is a readable, well-formed file or a real directory.
	StatusPresent = "present"
	// StatusMissing is an absent path.
	StatusMissing = "missing"
	// StatusInvalid is a present file that fails to parse (carries Error).
	StatusInvalid = "invalid"
	// StatusNotADir is a regular file where a directory is expected.
	StatusNotADir = "not_a_dir"
	// StatusPresentEmpty is a well-formed registry whose scenarios map is
	// empty — distinct from missing and from a populated present.
	StatusPresentEmpty = "present_empty"
	// StatusAmbiguous is used where two artifacts collide.
	StatusAmbiguous = "ambiguous"
	// StatusUnknown is the applied fallback when the story cannot be
	// counted; the concrete cause is carried in Applied.Reason.
	StatusUnknown = "unknown"
)

// Epic parse statuses (plan §3.4).
const (
	// EpicParseable is a well-formed epic document.
	EpicParseable = "parseable"
	// EpicInvalid is an unparseable epic file.
	EpicInvalid = "invalid"
)

// Story created statuses (plan §3.4).
const (
	// CreatedOne resolves to exactly one parseable story file.
	CreatedOne = "one"
	// CreatedMissing resolves to no story file.
	CreatedMissing = "missing"
	// CreatedAmbiguous resolves to more than one story file.
	CreatedAmbiguous = "ambiguous"
	// CreatedInvalid resolves to one unparseable story file.
	CreatedInvalid = "invalid"
)

// Applied statuses and unknown reasons (plan §3.4). Apply ELIGIBILITY is
// separate from parseability: a story can parse yet still be
// apply-ineligible (deprecated format, zero ACs, empty internal id).
const (
	// AppliedCounted means x/y lineage coverage is countable.
	AppliedCounted = "counted"
	// AppliedUnknown means coverage cannot be counted (see Reason).
	AppliedUnknown = "unknown"

	// ReasonMissing — no story file to count against.
	ReasonMissing = "missing"
	// ReasonAmbiguous — multiple story files match.
	ReasonAmbiguous = "ambiguous"
	// ReasonInvalid — the single story file does not parse.
	ReasonInvalid = "invalid"
	// ReasonDeprecatedFormat — the legacy scenarios.test_scenarios block.
	ReasonDeprecatedFormat = "deprecated_format"
	// ReasonNoAcceptanceCriteria — zero acceptance_criteria entries.
	ReasonNoAcceptanceCriteria = "no_acceptance_criteria"
	// ReasonEmptyInternalID — the story-file internal id is empty.
	ReasonEmptyInternalID = "empty_internal_id"
	// ReasonRegistryMissing — the scenarios registry us apply requires is
	// absent, so an otherwise-eligible story cannot be counted (rather
	// than silently rendered counted 0/y). us apply copies the required
	// registry first and fails when it is missing (us_apply.go).
	ReasonRegistryMissing = "registry_missing"
	// ReasonRegistryInvalid — the scenarios registry is present but
	// unparseable, so lineage coverage cannot be counted honestly.
	ReasonRegistryInvalid = "registry_invalid"
)

// RefinedNotRecorded is the only refined value in v1 (plan §2/§3.4). The
// UI renders it as the literal text "not recorded".
const RefinedNotRecorded = "not_recorded"

// Snapshot is the complete folder inventory the remote uploads. Its JSON
// shape is the binding contract documented in the package comment.
// Documents is a key→status map so the UI renders each chip's data-status
// directly; DocumentErrors carries the parse error for `invalid` chips.
//
// The size-discipline fields (plan §1a) describe how far the request-budget
// degrade ladder was walked to make the SERIALIZED snapshot fit: Totals
// always carries the true global counts (surviving epic-entry truncation);
// SnapshotTruncated / StoriesOmitted / Unavailable name the mode(s) reached
// and drive the visible inventory-truncated-banner.
type Snapshot struct {
	Documents                  map[string]string `json:"documents"`
	DocumentErrors             map[string]string `json:"document_errors,omitempty"`
	ArchitecturePathMismatch   bool              `json:"architecture_path_mismatch"`
	ConfiguredArchitecturePath string            `json:"configured_architecture_path,omitempty"`
	CanonicalArchitecturePath  string            `json:"canonical_architecture_path,omitempty"`
	Epics                      []Epic            `json:"epics"`
	Totals                     Totals            `json:"totals"`
	SnapshotTruncated          bool              `json:"snapshot_truncated,omitempty"`
	StoriesOmitted             int               `json:"stories_omitted,omitempty"`
	Unavailable                string            `json:"unavailable,omitempty"`
}

// Totals are the true global counts, computed over the full scan BEFORE any
// degradation, so they survive epic-entry truncation down to the floor.
type Totals struct {
	Epics           int `json:"epics"`
	DeclaredStories int `json:"declared_stories"`
}

// Epic is one epic row: its filename identity, document identity, and the
// per-row identity divergences the UI flags. Title is the epic document's
// name: (empty when unparseable; the UI falls back to the filename).
type Epic struct {
	File                 string  `json:"file"`
	Number               int     `json:"number"`
	Status               string  `json:"status"`
	Title                string  `json:"title,omitempty"`
	DocID                int     `json:"doc_id"`
	IDMismatch           bool    `json:"id_mismatch"`
	DuplicateNumber      bool    `json:"duplicate_number"`
	NoncanonicalFilename bool    `json:"noncanonical_filename"`
	Error                string  `json:"error,omitempty"`
	Stories              []Story `json:"stories"`
}

// Story is one epic story row with its full identity tuple, created /
// applied / refined cells, identity flags, and the review-modal content
// (plan §1/§1b). Declared* / DeclaredContent come from the epic declaration
// and back the "every story openable" fallback; Content / Raw come from the
// resolved story file; the omission fields record what the degrade ladder
// dropped to fit the request budget (plan §1a).
type Story struct {
	CreateID        string     `json:"create_id"`
	EpicNumber      int        `json:"epic_number"`
	Position        int        `json:"position"`
	DeclaredID      string     `json:"declared_id"`
	FileID          string     `json:"file_id,omitempty"`
	Created         string     `json:"created"`
	Applied         Applied    `json:"applied"`
	Refined         string     `json:"refined"`
	Flags           StoryFlags `json:"flags"`
	DeclaredTitle   string     `json:"declared_title,omitempty"`
	DeclaredStatus  string     `json:"declared_status,omitempty"`
	MatchCount      int        `json:"match_count,omitempty"`
	SourceFile      string     `json:"source_file,omitempty"`
	Error           string     `json:"error,omitempty"`
	Content         *Content   `json:"content,omitempty"`
	DeclaredContent *Content   `json:"declared_content,omitempty"`
	Raw             string     `json:"raw,omitempty"`
	RawTruncated    bool       `json:"raw_truncated,omitempty"`
	RawOmitted      bool       `json:"raw_omitted,omitempty"`
	ContentOmitted  bool       `json:"content_omitted,omitempty"`
	OmissionReason  string     `json:"omission_reason,omitempty"`
}

// Applied is the story-applied cell: either a countable x/y or an
// unknown with a concrete reason.
type Applied struct {
	Status  string `json:"status"`
	Applied int    `json:"applied,omitempty"`
	Total   int    `json:"total,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// StoryFlags are the independent identity flags a story row can raise.
// Each is omitempty so a clean row stays compact under the request budget
// (the UI renders a chip only for a flag that is true).
type StoryFlags struct {
	DuplicateDeclaredID bool `json:"duplicate_declared_id,omitempty"`
	IDMismatch          bool `json:"id_mismatch,omitempty"`
	DeprecatedFormat    bool `json:"deprecated_format,omitempty"`
	NoACs               bool `json:"no_acs,omitempty"`
	EmptyInternalID     bool `json:"empty_internal_id,omitempty"`
}
