package drill

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func (a *Audit) references(value any, evidence, tree string, roots []string, verdict string) {
	switch value := value.(type) {
	case string:
		a.textReferences(value, evidence, tree, roots, verdict)
	case []any:
		for _, item := range value {
			a.references(item, evidence, tree, roots, verdict)
		}
	case map[string]any:
		name, _ := value["name"].(string)
		if name == "" {
			name, _ = value["tool_name"].(string)
		}

		if name != "" {
			a.toolReference(name, value, evidence, tree, roots, verdict)

			return
		}

		for _, item := range value {
			a.references(item, evidence, tree, roots, verdict)
		}
	}
}

func (a *Audit) toolReference(name string,
	value map[string]any,
	evidence,
	tree string,
	roots []string,
	verdict string) {
	if explicit := decision(value); explicit != unknownValue {
		verdict = explicit
	}

	raw, err := json.Marshal(value)
	if err != nil {
		a.Unknown = append(a.Unknown, err.Error())

		return
	}

	targets := []string{}
	collectPaths(value, "", &targets)

	if len(targets) == 0 {
		targets = append(targets, "")
	}

	for _, target := range targets {
		a.addAccess(name, target, string(raw), evidence, tree, roots, verdict)
	}
}

func collectPaths(value any, key string, targets *[]string) {
	switch value := value.(type) {
	case map[string]any:
		for name, item := range value {
			collectPaths(item, name, targets)
		}
	case []any:
		for _, item := range value {
			collectPaths(item, key, targets)
		}
	case string:
		var nested any
		if json.Unmarshal([]byte(value), &nested) == nil {
			collectPaths(nested, key, targets)

			return
		}

		if pathKey(key) {
			*targets = append(*targets, value)

			return
		}

		*targets = append(*targets, pathReferences(value)...)
	}
}

func pathReferences(text string) []string {
	const pattern = `https?://[^\s"'<>]+|(?:/|\.{1,2}/|` +
		`(?:docs|services|tests|pkg|scripts|tmp|true-bdd|templates)/)[^\s"'<>;,\]\[{}()]+`

	return regexp.MustCompile(pattern).FindAllString(text, -1)
}

func networkTool(name string) bool {
	lower := strings.ToLower(name)

	return strings.Contains(lower,
		"fetch") || strings.Contains(lower,
		"sourcegraph") || strings.Contains(lower,
		"context7")
}

func (a *Audit) addAccess(tool, target, raw, evidence, tree string, roots []string, verdict string) {
	group := unknownValue
	if networkTool(tool) || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
		group = "network"
	} else if target != "" {
		if !filepath.IsAbs(target) {
			target = filepath.Join(tree, target)
		}

		target = resolvePath(target)
		group = pathGroup(target, tree, roots)
	}

	a.Entries = append(a.Entries,
		Access{Evidence: evidence,
			Tool:     tool,
			Target:   target,
			Group:    group,
			Decision: verdict,
			Raw:      raw})
}

func resolvePath(path string) string {
	clean := filepath.Clean(path)

	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved
	}

	parent := filepath.Dir(clean)
	if parent == clean {
		return clean
	}

	return filepath.Join(resolvePath(parent), filepath.Base(clean))
}

func (a *Audit) String() string {
	return fmt.Sprintf("%d references, %d evidence files, %d limitations", len(a.Entries), len(a.Files), len(a.Unknown))
}

func (a *Audit) textReferences(value, evidence, tree string, roots []string, verdict string) {
	var nested any
	if json.Unmarshal([]byte(value), &nested) == nil {
		a.references(nested, evidence, tree, roots, verdict)

		return
	}

	for _, target := range pathReferences(value) {
		a.addAccess("reference", target, value, evidence, tree, roots, verdict)
	}

	if networkTool(value) {
		a.addAccess("network reference", "", value, evidence, tree, roots, verdict)
	}
}

func pathKey(key string) bool {
	switch key {
	case "file_path", "path", "target_path", "url", "directory":
		return true
	default:
		return false
	}
}

func pathGroup(target, tree string, roots []string) string {
	if within(target, resolvePath(tree)) {
		return "inside"
	}

	for _, root := range roots {
		if root != "" && within(target, resolvePath(root)) {
			return "allowed_root"
		}
	}

	return "outside"
}
