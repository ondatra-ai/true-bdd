package commands

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ErrTermDocumentUnparseable is a fix that left a document's scratch copy
// unreadable: the document itself is left as it was.
var ErrTermDocumentUnparseable = errors.New("the fix left the document unreadable, so it is unchanged")

// ErrRegistryScenariosChanged is a fix that added or removed a Scenario,
// where it may only rewrite the one it was asked to.
var ErrRegistryScenariosChanged = errors.New("the fix added or removed a Scenario of the Registry")

// termDocumentKeys are the documents a closure fix may write: the
// catalogue and the glossary live in the product document, the word
// mapping in the architecture document.
func termDocumentKeys() []string {
	return []string{docs.KeyProduct, docs.KeyArchitectureYAML}
}

// seedTermDocuments copies each term document the host has into the run's
// tmp dir, where the fix applier may edit it; a document the host lacks
// is one no fix can write.
func seedTermDocuments(resolver *docs.Resolver, tmpDir string) ([]template.FixDocument, error) {
	seeded := make([]template.FixDocument, 0, len(termDocumentKeys()))

	for _, key := range termDocumentKeys() {
		path, err := resolver.Resolve(key)
		if err != nil {
			slog.Info("Term document not seeded", "document", key, "reason", err)

			continue
		}

		document, err := seedFixDocument(key, path, tmpDir)
		if err != nil {
			return nil, err
		}

		seeded = append(seeded, document)
	}

	return seeded, nil
}

// seedInterfaceDocuments copies each Public interface document the
// architecture names into tmp, keyed by its declared path, for a
// public-interface or phrase-mappings fix to extend.
func seedInterfaceDocuments(resolver *docs.Resolver, tmpDir string) ([]template.FixDocument, error) {
	architecturePath, err := resolver.Resolve(docs.KeyArchitectureYAML)
	if err != nil {
		return nil, nil //nolint:nilerr // seedTermDocuments already logged the missing architecture.
	}

	declared, err := architecture.DeclaredInterfaceDocuments(architecturePath)
	if err != nil {
		return nil, fmt.Errorf("list the Public interface documents: %w", err)
	}

	seeded := make([]template.FixDocument, 0, len(declared))

	for idx, document := range declared {
		scratch := filepath.Join(tmpDir, fmt.Sprintf("interface-%d-%s", idx+1, filepath.Base(document.Path)))

		err = disk.Copy(scratch, document.Path, disk.Shared)
		if err != nil {
			return nil, fmt.Errorf("seed the scratch copy of %s: %w", document.Path, err)
		}

		slog.Info("Seeded scratch document", "document", document.Path, "from", document.Path, "to", scratch)

		seeded = append(seeded, template.FixDocument{Key: document.Path, Path: document.Path, Scratch: scratch})
	}

	return seeded, nil
}

// seedFixDocument copies one document into the run's tmp dir as the
// working copy a fix edits.
func seedFixDocument(key, path, tmpDir string) (template.FixDocument, error) {
	scratch := filepath.Join(tmpDir, key+".yaml")

	err := disk.Copy(scratch, path, disk.Shared)
	if err != nil {
		return template.FixDocument{}, fmt.Errorf("seed the scratch copy of %s: %w", path, err)
	}

	slog.Info("Seeded scratch document", "document", key, "from", path, "to", scratch)

	return template.FixDocument{Key: key, Path: path, Scratch: scratch}, nil
}

// commitFixDocuments replaces each document whose scratch copy the fix
// changed, once that copy reads as the document it replaces, and reports
// whether the Registry was one of them. The copy stays, equal to the document.
func commitFixDocuments(item *template.ScenarioCheckData) (bool, error) {
	documents := slices.Clone(item.TermDocuments)
	if item.RegistryDocument != nil {
		documents = append(documents, *item.RegistryDocument)
	}

	changed, err := commitDocuments(documents, registryIDs(item.Registry))
	if err != nil {
		return false, err
	}

	return slices.Contains(changed, docs.KeyScenariosYAML), nil
}

// commitDocuments commits each working copy the fix changed and lists the
// keys of those documents; ids are the Registry's, which its copy keeps.
func commitDocuments(documents []template.FixDocument, ids []string) ([]string, error) {
	changed := make([]string, 0, len(documents))

	for _, document := range documents {
		committed, err := commitFixDocument(document, ids)
		if err != nil {
			return nil, err
		}

		if committed {
			changed = append(changed, document.Key)
		}
	}

	if len(changed) == 0 {
		slog.Warn("The fix changed no document")
	}

	return changed, nil
}

// commitFixDocument commits one scratch copy, reporting whether it differed.
func commitFixDocument(document template.FixDocument, ids []string) (bool, error) {
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

	err = readableFixDocument(document.Key, document.Scratch, edited, ids)
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

// readableFixDocument holds an edited copy to what its readers need: a
// YAML mapping, and the parts of it the walk loads.
func readableFixDocument(key, scratch string, content []byte, ids []string) error {
	var mapping map[string]any

	err := yaml.Unmarshal(content, &mapping)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	switch key {
	case docs.KeyProduct:
		return readableProductDocument(scratch)
	case docs.KeyArchitectureYAML:
		_, err = architecture.LoadTermMappings(scratch)
		if err != nil {
			return fmt.Errorf("load the word mapping: %w", err)
		}
	case docs.KeyScenariosYAML:
		return readableRegistry(scratch, ids)
	}

	return nil
}

// readableProductDocument holds an edited product document to a catalogue
// and a glossary the checks load.
func readableProductDocument(scratch string) error {
	_, err := product.LoadCatalogue(scratch)
	if err != nil {
		return fmt.Errorf("load the catalogue: %w", err)
	}

	_, err = product.LoadGlossary(scratch)
	if err != nil {
		return fmt.Errorf("load the glossary: %w", err)
	}

	return nil
}

// readableRegistry holds an edited Registry to one that loads and keys
// the same Scenarios: a fix rewrites a Scenario, never adds or drops one.
func readableRegistry(scratch string, ids []string) error {
	scenarios, err := registry.NewRegistryLoader().Load(scratch)
	if err != nil {
		return fmt.Errorf("load the registry: %w", err)
	}

	edited := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		edited = append(edited, scenario.ID)
	}

	if !slices.Equal(edited, ids) {
		return fmt.Errorf("%w: %v became %v", ErrRegistryScenariosChanged, ids, edited)
	}

	return nil
}

// registryIDs lists the ids of the Registry as the walk loaded it, in the
// loader's ascending order.
func registryIDs(scenarios []*template.ScenarioCheckData) []string {
	ids := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		ids = append(ids, scenario.ID)
	}

	return ids
}

// commitInterfaceFix commits the architecture's working copy and the
// interface documents the fix changed, documents first, once each parses
// in the form of the kind the edited architecture declares for it.
func commitInterfaceFix(architectureDocument template.FixDocument, interfaceDocuments []template.FixDocument) error {
	_, err := architecture.LoadInterfaces(architectureDocument.Scratch, ".")
	if err != nil {
		return fmt.Errorf("validate Public interface fix: %w", err)
	}

	_, err = architecture.LoadPhraseMappings(architectureDocument.Scratch)
	if err != nil {
		return fmt.Errorf("validate Phrase mapping fix: %w", err)
	}

	declared, err := architecture.DeclaredInterfaceDocuments(architectureDocument.Scratch)
	if err != nil {
		return fmt.Errorf("validate Public interface fix: %w", err)
	}

	kinds := make(map[string]string, len(declared))
	for _, document := range declared {
		kinds[document.Path] = document.Kind
	}

	for _, document := range interfaceDocuments {
		err = checkInterfaceWorkingCopy(document, kinds[document.Path])
		if err != nil {
			return err
		}
	}

	for _, document := range interfaceDocuments {
		_, err = commitFixDocument(document, nil)
		if err != nil {
			return err
		}
	}

	_, err = commitFixDocument(architectureDocument, nil)

	return err
}

// checkInterfaceWorkingCopy holds a changed interface document working copy
// to the form of its kind; an unchanged one needs no check.
func checkInterfaceWorkingCopy(document template.FixDocument, kind string) error {
	edited, err := disk.Read(document.Scratch)
	if err != nil {
		return fmt.Errorf("read the scratch copy of %s: %w", document.Path, err)
	}

	current, err := disk.Read(document.Path)
	if err != nil {
		return fmt.Errorf("read %s: %w", document.Path, err)
	}

	if bytes.Equal(edited, current) {
		return nil
	}

	if kind == "" {
		return fmt.Errorf("%s: %w: no Public interface declaration names it any more",
			document.Path, ErrTermDocumentUnparseable)
	}

	err = architecture.CheckInterfaceDocument(kind, edited, document.Path, ".")
	if err != nil {
		return fmt.Errorf("%s: %w: %w", document.Path, ErrTermDocumentUnparseable, err)
	}

	return nil
}
