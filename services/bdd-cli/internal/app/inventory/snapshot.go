// Package inventory is the host-folder scanner (plan §3.4): it reports which
// of the documents the engine reads are present and well-formed (config,
// product, architecture, registry, stories directory, checklists), mirroring
// the engine loader semantics without constructing a bootstrap container, so
// it runs honestly in a bare or degraded folder and never panics on malformed
// input. The remote uploads the resulting Snapshot; a relay server stores it
// opaquely.
//
// # Snapshot JSON schema
//
// documents is a key→status map; document_errors carries the parse error
// for the `invalid` chips only.
//
//	{
//	  "documents": {"<key>": "<Status* const>", ...},       // per-key enum: Status* consts below
//	  "document_errors": {"config": "yaml: line 3: ..."},   // invalid chips only
//	  "architecture_path_mismatch": bool, "configured/canonical_architecture_path": "...",
//	  "snapshot_truncated": bool, "unavailable": "limit_too_small"
//	}
package inventory

// Document / directory / checklist chip statuses (plan §3.4).
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
)

// Snapshot is the complete folder inventory the remote uploads (JSON shape
// is the package doc's contract). SnapshotTruncated/Unavailable name which
// degrade mode the request budget forced.
type Snapshot struct {
	Documents                  map[string]string `json:"documents"`
	DocumentErrors             map[string]string `json:"document_errors,omitempty"`
	ArchitecturePathMismatch   bool              `json:"architecture_path_mismatch"`
	ConfiguredArchitecturePath string            `json:"configured_architecture_path,omitempty"`
	CanonicalArchitecturePath  string            `json:"canonical_architecture_path,omitempty"`
	SnapshotTruncated          bool              `json:"snapshot_truncated,omitempty"`
	Unavailable                string            `json:"unavailable,omitempty"`
}
