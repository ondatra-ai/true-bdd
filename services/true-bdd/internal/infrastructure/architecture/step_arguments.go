package architecture

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"gopkg.in/yaml.v3"
)

// LoadStepArguments preserves block source; flow documents and aliases render complete decoded values.
// An architecture without the optional section contributes no prompt bytes.
func LoadStepArguments(path string) (string, error) {
	data, err := disk.Read(path)
	if err != nil {
		return "", fmt.Errorf("read step arguments: %w", err)
	}

	var document yaml.Node

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return "", fmt.Errorf("parse step arguments: %w", err)
	}

	if len(document.Content) == 0 {
		return "", nil
	}

	root := document.Content[0]
	lines := strings.SplitAfter(string(data), "\n")

	for index := 0; index+1 < len(root.Content); index += 2 {
		key := root.Content[index]
		if key.Value != "step_arguments" {
			continue
		}

		value := root.Content[index+1]
		if root.Style&yaml.FlowStyle != 0 || stepArgumentAliases(value) {
			return renderStepArguments(value)
		}

		end := len(lines)
		if index+2 < len(root.Content) {
			end = root.Content[index+2].Line - 1
		}

		return strings.Join(lines[key.Line-1:end], ""), nil
	}

	return "", nil
}

func stepArgumentAliases(node *yaml.Node) bool {
	if node.Kind == yaml.AliasNode {
		return true
	}

	for _, child := range node.Content {
		if stepArgumentAliases(child) {
			return true
		}
	}

	return false
}

func renderStepArguments(node *yaml.Node) (string, error) {
	var arguments any

	err := node.Decode(&arguments)
	if err != nil {
		return "", fmt.Errorf("decode step arguments: %w", err)
	}

	rendered, err := yaml.Marshal(map[string]any{"step_arguments": arguments})
	if err != nil {
		return "", fmt.Errorf("render step arguments: %w", err)
	}

	return string(rendered), nil
}
