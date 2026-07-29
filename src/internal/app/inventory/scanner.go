package inventory

import "path/filepath"

// Scan builds the complete folder inventory (plan §3.4). folder must be
// the canonical (realpath) host folder. Scan never fails: every degraded
// state (missing config, unparseable epic, ambiguous story, empty
// registry, non-default architecture path) resolves to an explicit status
// in the returned Snapshot rather than an error, so the remote can always
// upload an honest picture.
func Scan(folder string) Snapshot {
	cfg := resolveConfig(folder)
	lineage := loadLineageIndex(filepath.Join(folder, registryRelPath))
	documents, documentErrors := scanDocuments(folder, cfg)

	return Snapshot{
		Documents:                  documents,
		DocumentErrors:             documentErrors,
		ArchitecturePathMismatch:   cfg.architectureMismatch,
		ConfiguredArchitecturePath: cfg.architectureRel,
		CanonicalArchitecturePath:  defaultArchitecturePath,
		Epics: scanEpics(epicScanInput{
			epicsDir:   filepath.Join(folder, cfg.epicsRel),
			storiesDir: filepath.Join(folder, cfg.storiesRel),
			storiesRel: cfg.storiesRel,
			lineage:    lineage,
		}),
	}
}
