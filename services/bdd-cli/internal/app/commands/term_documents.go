package commands

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// ErrTermDocumentUnparseable is a fix that left a term document's scratch
// copy unreadable: the document itself is left as it was.
var ErrTermDocumentUnparseable = errors.New("the fix left the document unreadable, so it is unchanged")

// termDocumentKeys are the documents a `scen check` fix may write: the
// catalogue and the glossary both live in the product document.
func termDocumentKeys() []string {
	return []string{docs.KeyProduct}
}

// seedTermDocuments copies each term document the host has into the run's
// tmp dir, where the fix applier may edit it; a document the host lacks
// is one no fix can write.
func seedTermDocuments(resolver *docs.Resolver, tmpDir string) ([]template.TermDocument, error) {
	seeded := make([]template.TermDocument, 0, len(termDocumentKeys()))

	for _, key := range termDocumentKeys() {
		path, err := resolver.Resolve(key)
		if err != nil {
			slog.Info("Term document not seeded", "document", key, "reason", err)

			continue
		}

		scratch := filepath.Join(tmpDir, key+".yaml")

		err = disk.Copy(scratch, path, disk.Shared)
		if err != nil {
			return nil, fmt.Errorf("seed the scratch copy of %s: %w", path, err)
		}

		slog.Info("Seeded scratch document", "document", key, "from", path, "to", scratch)

		seeded = append(seeded, template.TermDocument{Key: key, Path: path, Scratch: scratch})
	}

	return seeded, nil
}

// commitTermDocuments replaces each term document whose scratch copy the
// fix changed, once that copy reads as the document it replaces. The
// copy stays in place, equal to the document, for the next fix.
func commitTermDocuments(documents []template.TermDocument) error {
	committed := 0

	for _, document := range documents {
		changed, err := commitTermDocument(document)
		if err != nil {
			return err
		}

		if changed {
			committed++
		}
	}

	if committed == 0 {
		slog.Warn("The fix changed no term document")
	}

	return nil
}

// commitTermDocument commits one scratch copy, reporting whether it differed.
func commitTermDocument(document template.TermDocument) (bool, error) {
	edited, err := disk.Read(document.Scratch)
	if err != nil {
		return false, fmt.Errorf("read the scratch copy of %s: %w", document.Path, err)
	}

	current, err := disk.Read(document.Path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", document.Path, err)
	}

	if bytes.Equal(edited, current) {
		return false, nil
	}

	err = readableTermDocument(document.Key, document.Scratch, edited)
	if err != nil {
		return false, fmt.Errorf("%s: %w: %w", document.Path, ErrTermDocumentUnparseable, err)
	}

	err = disk.Copy(document.Path, document.Scratch, disk.Shared)
	if err != nil {
		return false, fmt.Errorf("replace %s with its fixed copy: %w", document.Path, err)
	}

	slog.Info("Fix recorded in the document", "document", document.Key, "path", document.Path)

	return true, nil
}

// readableTermDocument holds an edited copy to what its readers need: a
// YAML mapping, and for the product document a catalogue the checks load.
func readableTermDocument(key, scratch string, content []byte) error {
	var mapping map[string]any

	err := yaml.Unmarshal(content, &mapping)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if key != docs.KeyProduct {
		return nil
	}

	_, err = product.LoadCatalogue(scratch)
	if err != nil {
		return fmt.Errorf("load the catalogue: %w", err)
	}

	return nil
}
