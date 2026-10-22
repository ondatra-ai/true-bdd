package commands

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// fixLedger holds each document a `--fix` run may write as it read before
// the walk, so a run that never converges can put every one back.
type fixLedger struct {
	documents []template.FixDocument
	originals [][]byte
}

// keep records a document's bytes as they stand before any fix.
func (l *fixLedger) keep(documents ...template.FixDocument) error {
	for _, document := range documents {
		original, err := disk.Read(document.Path)
		if err != nil {
			return fmt.Errorf("read %s before the walk: %w", document.Path, err)
		}

		l.documents = append(l.documents, document)
		l.originals = append(l.originals, original)
	}

	return nil
}

// restore puts back every document a fix changed: a fix stands only once
// a walk has confirmed it, as the exit choice leaves them.
func (l *fixLedger) restore() error {
	var failed []error

	for idx, document := range l.documents {
		current, err := disk.Read(document.Path)
		if err == nil && bytes.Equal(current, l.originals[idx]) {
			continue
		}

		err = disk.Write(document.Path, l.originals[idx], disk.Shared)
		if err != nil {
			failed = append(failed, fmt.Errorf("put %s back as it was: %w", document.Path, err))

			continue
		}

		slog.Info("Fix undone in the document", "document", document.Key, "path", document.Path)
	}

	return errors.Join(failed...)
}
