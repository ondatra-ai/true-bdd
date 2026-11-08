package verification

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"gopkg.in/yaml.v3"
)

func retainInterfaceReferences(original, selected map[string]any, origin []string) error {
	path := ""
	if len(origin) > 0 {
		path = origin[0]
	}

	documents := map[string]map[string]any{path: original}
	selections := map[string]map[string]any{path: selected}

	processed := make(map[string]bool)
	for {
		pending := pendingReferences(selections, processed)
		if len(pending) == 0 {
			break
		}

		for _, reference := range pending {
			processed[reference.document+"#"+reference.target] = true

			err := retainInterfaceReference(reference, documents, selections)
			if err != nil {
				return err
			}
		}
	}

	for name, document := range selections {
		if name != path {
			putEntry(selected, []string{"referenced_documents", name}, document)
		}
	}

	return nil
}

type interfaceReference struct{ document, target string }

func pendingReferences(selections map[string]map[string]any, processed map[string]bool) []interfaceReference {
	pending := make([]interfaceReference, 0)

	for document, selection := range selections {
		walkReferences(selection, func(target string) {
			if !processed[document+"#"+target] {
				pending = append(pending, interfaceReference{document, target})
			}
		})
	}

	return pending
}

func retainInterfaceReference(reference interfaceReference, documents, selections map[string]map[string]any) error {
	file, pointer, _ := strings.Cut(reference.target, "#")

	path := reference.document
	if file != "" {
		path = filepath.Clean(filepath.Join(filepath.Dir(path), file))
	}

	original := documents[path]
	if original == nil {
		content, err := disk.Read(path)
		if err != nil {
			return fmt.Errorf("read referenced Public interface %s: %w", path, err)
		}

		err = yaml.Unmarshal(content, &original)
		if err != nil {
			return fmt.Errorf("parse referenced Public interface %s: %w", path, err)
		}

		documents[path] = original
	}

	parts := referenceParts(pointer)

	entry, found := nestedEntry(original, parts)
	if !found {
		return fmt.Errorf("%w: verification reference %s is missing", architecture.ErrInterfaceDocument, reference.target)
	}

	if selections[path] == nil {
		selections[path] = make(map[string]any)
	}

	if len(parts) == 0 {
		selections[path] = original
	} else {
		putEntry(selections[path], parts, entry)
	}

	return nil
}

func referenceParts(pointer string) []string {
	if pointer == "" {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for index, part := range parts {
		parts[index] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}

	return parts
}
