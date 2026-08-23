package inventory

import "path/filepath"

// Scan builds the complete folder inventory (plan §3.4), fit to the
// env-provided request budget (TRUE_BDD_INVENTORY_BUDGET_BYTES; unset means
// unlimited); folder must be canonical (realpath). Scan never fails.
func Scan(folder string) Snapshot {
	return ScanWithBudget(folder, envBudget())
}

// ScanWithBudget fits the snapshot so its SERIALIZED size does not exceed
// budget bytes (plan §1a); budget <= 0 means unlimited. budget > 0 is
// INCREMENTAL (finding 2), bounding retained memory (see scanner_memory_test.go).
func ScanWithBudget(folder string, budget int) Snapshot {
	cfg := resolveConfig(folder)
	lineage := loadLineageIndex(filepath.Join(folder, cfg.registryRel))
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
