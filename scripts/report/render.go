package report

import (
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Render folds run out of the log at path, narrates the table and leaves it at
// dest. Nothing here stops a caller: a report is what a run says about itself
// once the work is done, and failing to produce one changes none of it.
func Render(path, run, dest string) {
	folded, err := Fold(path, run)
	if err != nil {
		slog.Warn("could not read this run's log back", "error", err)

		return
	}

	// Narrated AFTER the fold, and carrying none of the contract keys, so
	// re-parsing this file later cannot mistake the report for operations.
	folded.Narrate()

	err = disk.Write(dest, []byte(folded.Table()), disk.Shared)
	if err != nil {
		slog.Warn("could not write the report", "path", dest, "error", err)
	}
}

// Latest is the run the log's last record belongs to, for a reader who wants
// the most recent one and does not know its id.
func Latest(path string) (string, error) {
	raw, err := disk.Read(path)
	if err != nil {
		return "", err
	}

	records := decode(raw, "")

	for index := len(records) - 1; index >= 0; index-- {
		if records[index].Run != "" {
			return records[index].Run, nil
		}
	}

	return "", errNoRecords
}
