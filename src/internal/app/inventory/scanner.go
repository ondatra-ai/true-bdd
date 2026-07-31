package inventory

import "path/filepath"

// Scan builds the complete folder inventory (plan §3.4), fitting it to the
// env-provided request budget (TRUE_BDD_INVENTORY_BUDGET_BYTES; unset means
// unlimited). folder must be the canonical (realpath) host folder. Scan
// never fails: every degraded state (missing config, unparseable epic,
// ambiguous story, empty registry, non-default architecture path, an
// over-budget snapshot) resolves to an explicit status in the returned
// Snapshot rather than an error, so the remote can always upload an honest
// picture.
func Scan(folder string) Snapshot {
	return ScanWithBudget(folder, envBudget())
}

// ScanWithBudget scans the folder and fits the snapshot so its SERIALIZED
// size does not exceed budget bytes (plan §1a). budget <= 0 means unlimited
// (no fitting). The remote passes the budget it derives from the server's
// negotiated inventory limit; a 413 drives a strictly-smaller re-scan.
//
// For budget <= 0 (unlimited) the scan retains every enriched epic/story —
// there is no server limit to fit. For budget > 0 the scan is INCREMENTAL
// (finding 2): epic docs are parsed cheaply first, then story rows are read
// and fitted ONE AT A TIME, so aggregate retained memory is bounded by the
// budget plus one in-flight story file rather than the whole folder.
func ScanWithBudget(folder string, budget int) Snapshot {
	cfg := resolveConfig(folder)
	lineage := loadLineageIndex(filepath.Join(folder, registryRelPath))
	documents, documentErrors := scanDocuments(folder, cfg)

	input := epicScanInput{
		epicsDir:   filepath.Join(folder, cfg.epicsRel),
		storiesDir: filepath.Join(folder, cfg.storiesRel),
		storiesRel: cfg.storiesRel,
		lineage:    lineage,
	}

	base := Snapshot{
		Documents:                  documents,
		DocumentErrors:             documentErrors,
		ArchitecturePathMismatch:   cfg.architectureMismatch,
		ConfiguredArchitecturePath: cfg.architectureRel,
		CanonicalArchitecturePath:  defaultArchitecturePath,
	}

	if budget <= 0 {
		epics := scanEpics(input)
		base.Epics = epics
		base.Totals = Totals{Epics: len(epics), DeclaredStories: countDeclaredStories(epics)}

		return base
	}

	return scanEpicsFitted(input, base, budget)
}
