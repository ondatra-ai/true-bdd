package verification

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"gopkg.in/yaml.v3"
)

const interfaceExtensions = "extensions"

// InterfaceEntries selects complete entries named by a binding, retaining their document structure.
func InterfaceEntries(document string, references []architecture.InterfaceReference, origin ...string) (string, error) {
	if document == "" {
		return "", nil
	}

	var original map[string]any

	err := yaml.Unmarshal([]byte(document), &original)
	if err != nil {
		return "", fmt.Errorf("parse verification interface: %w", err)
	}

	selected := make(map[string]any)

	for _, reference := range references {
		selector, path := interfaceSelector(reference)
		if reference.OperationID != "" {
			if !selectOperation(original, selected, reference.OperationID) {
				return "", fmt.Errorf("%w: verification operation %s is missing",
					architecture.ErrInterfaceDocument, reference.OperationID)
			}

			continue
		}

		entry, found := nestedEntry(original, append(path, selector))
		if !found {
			return "", fmt.Errorf("%w: verification reference %s is missing", architecture.ErrInterfaceDocument, reference)
		}

		putEntry(selected, append(path, selector), entry)
	}

	err = retainInterfaceReferences(original, selected, origin)
	if err != nil {
		return "", err
	}

	encoded, err := yaml.Marshal(selected)
	if err != nil {
		return "", fmt.Errorf("render verification interface: %w", err)
	}

	return string(encoded), nil
}

func interfaceSelector(reference architecture.InterfaceReference) (string, []string) {
	switch {
	case reference.Command != "":
		return reference.Command, []string{"commands"}
	case reference.File != "":
		return reference.File, []string{interfaceExtensions, "files"}
	case reference.LogRecord != "":
		return reference.LogRecord, []string{interfaceExtensions, "log_records"}
	case reference.OutgoingCall != "":
		return reference.OutgoingCall, []string{interfaceExtensions, "outgoing_calls"}
	default:
		return "", nil
	}
}

func nestedEntry(document map[string]any, path []string) (any, bool) {
	var current any = document
	for _, key := range path {
		mapping, mappingFound := current.(map[string]any)
		if !mappingFound {
			return nil, false
		}

		var found bool

		current, found = mapping[key]
		if !found {
			return nil, false
		}
	}

	return current, true
}

func putEntry(document map[string]any, path []string, value any) {
	current := document
	for _, key := range path[:len(path)-1] {
		nested, ok := current[key].(map[string]any)
		if !ok {
			nested = make(map[string]any)
			current[key] = nested
		}

		current = nested
	}

	current[path[len(path)-1]] = value
}

func selectOperation(original, selected map[string]any, operation string) bool {
	found := false

	paths, _ := original["paths"].(map[string]any)
	for path, value := range paths {
		methods, _ := value.(map[string]any)
		for method, value := range methods {
			entry, ok := value.(map[string]any)
			if !ok || entry["operationId"] != operation {
				continue
			}

			found = true

			putEntry(selected, []string{"paths", path, method}, entry)

			for _, key := range []string{"parameters", "servers"} {
				if shared, found := methods[key]; found {
					putEntry(selected, []string{"paths", path, key}, shared)
				}
			}
		}
	}

	return found
}

func walkReferences(value any, visit func(string)) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if reference, ok := child.(string); key == "$ref" && ok {
				visit(reference)
			}

			walkReferences(child, visit)
		}
	case []any:
		for _, child := range value {
			walkReferences(child, visit)
		}
	}
}
