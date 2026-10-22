package codexrun

import (
	"os"
	"path/filepath"
	"reflect"
)

func (r Runner) recheck(record Record, resume bool, before, after []Record) error {
	path := r.file(record.Name, "run.json")
	if resume {
		current, err := load(path)
		if err != nil || !reflect.DeepEqual(current, record) {
			return refuse("run changed during validation")
		}
	} else {
		_, err := os.Stat(filepath.Dir(path))
		if !os.IsNotExist(err) {
			return refuse("run name already exists or cannot be checked")
		}
	}

	if !reflect.DeepEqual(relatedRecords(record, before), relatedRecords(record, after)) {
		return refuse("writer ownership changed during validation")
	}

	return nil
}

func relatedRecords(record Record, records []Record) []Record {
	var related []Record
	if !record.writer() {
		return related
	}

	for _, other := range records {
		if other.writer() && (other.Worktree == record.Worktree || other.Branch == record.Branch) {
			related = append(related, other)
		}
	}

	return related
}
