package inventory

// Scan builds the complete folder inventory (plan §3.4), fit to the
// env-provided request budget (TRUE_BDD_INVENTORY_BUDGET_BYTES; unset means
// unlimited); folder must be canonical (realpath). Scan never fails.
func Scan(folder string) Snapshot {
	return ScanWithBudget(folder, envBudget())
}

// ScanWithBudget fits the snapshot so its SERIALIZED size does not exceed
// budget bytes (plan §1a); budget <= 0 means unlimited.
func ScanWithBudget(folder string, budget int) Snapshot {
	cfg := resolveConfig(folder)
	documents, documentErrors := scanDocuments(folder, cfg)

	snapshot := Snapshot{
		Documents:                  documents,
		DocumentErrors:             documentErrors,
		ArchitecturePathMismatch:   cfg.architectureMismatch,
		ConfiguredArchitecturePath: cfg.architectureRel,
		CanonicalArchitecturePath:  defaultArchitecturePath,
	}

	if budget <= 0 || serializedLen(snapshot) <= budget {
		return snapshot
	}

	return floorSnapshot(snapshot, budget)
}
