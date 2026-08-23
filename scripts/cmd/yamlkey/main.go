// Command yamlkey prints one value out of a YAML document.
//
//	yamlkey true-bdd/true-bdd.yaml documents.product
//
// It exists for scripts/lint-schemas.sh, which needs to resolve
// `documents.<key>` to a path and has no other reason to hold a YAML parser.
// A missing key prints nothing and exits 0 — the caller's own test for
// "declared or not" is whether the line came back empty.
package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 3 { //nolint:mnd // the program name plus two arguments.
		fmt.Fprintln(os.Stderr, "usage: yamlkey <file.yaml> <dotted.path>")
		os.Exit(2) //nolint:mnd // 2 is a usage error, distinct from a lookup that found nothing.
	}

	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "yamlkey: %v\n", err)
		os.Exit(1)
	}

	var document any

	err = yaml.Unmarshal(raw, &document)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yamlkey: parsing %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	if value, ok := lookup(document, strings.Split(os.Args[2], ".")); ok {
		_, _ = fmt.Fprintln(os.Stdout, value)
	}
}

// lookup walks a dotted path, and reports whether it reached a scalar.
func lookup(node any, path []string) (string, bool) {
	for _, step := range path {
		mapping, ok := node.(map[string]any)
		if !ok {
			return "", false
		}

		if node, ok = mapping[step]; !ok {
			return "", false
		}
	}

	switch typed := node.(type) {
	case nil:
		return "", false
	case string:
		return typed, true
	case map[string]any, []any:
		return "", false
	default:
		return fmt.Sprint(typed), true
	}
}
