package architecture

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrRecordCommand distinguishes a malformed recording declaration from a missing command.
var ErrRecordCommand = errors.New(
	"testing.commands.record must be a string or {command: string, exclude: [string, ...]}")

const yamlStringTag = "!!str"

// UnmarshalYAML keeps command readers independent of the record declaration's form.
func (c *SuiteCommands) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Record yaml.Node `yaml:"record"`
		Replay string    `yaml:"replay"`
		Live   string    `yaml:"live"`
	}

	err := node.Decode(&raw)
	if err != nil {
		return fmt.Errorf("decode testing.commands: %w", err)
	}

	command, exclude, err := decodeRecordCommand(&raw.Record)
	if err != nil {
		return err
	}

	*c = SuiteCommands{Record: command, RecordExclude: exclude, Replay: raw.Replay, Live: raw.Live}

	return nil
}

func decodeRecordCommand(node *yaml.Node) (string, []string, error) {
	if node.Kind == 0 {
		return "", nil, nil
	}

	if node.Kind == yaml.AliasNode {
		return decodeRecordCommand(node.Alias)
	}

	if node.Kind == yaml.ScalarNode && node.Tag == yamlStringTag {
		return node.Value, nil, nil
	}

	if node.Kind != yaml.MappingNode {
		return "", nil, ErrRecordCommand
	}

	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index].Value
		if key != "command" && key != "exclude" {
			return "", nil, fmt.Errorf("%w: unknown key %q", ErrRecordCommand, key)
		}
	}

	return decodeRecordMapping(node)
}

func decodeRecordMapping(node *yaml.Node) (string, []string, error) {
	var raw struct {
		Command yaml.Node `yaml:"command"`
		Exclude yaml.Node `yaml:"exclude"`
	}

	err := node.Decode(&raw)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrRecordCommand, err)
	}

	if raw.Command.Kind != yaml.ScalarNode || raw.Command.Tag != yamlStringTag ||
		strings.TrimSpace(raw.Command.Value) == "" {
		return "", nil, ErrRecordCommand
	}

	exclude, err := decodeRecordExclude(&raw.Exclude)
	if err != nil {
		return "", nil, err
	}

	return raw.Command.Value, exclude, nil
}

func decodeRecordExclude(node *yaml.Node) ([]string, error) {
	if node.Kind == 0 {
		return nil, nil
	}

	if node.Kind != yaml.SequenceNode {
		return nil, ErrRecordCommand
	}

	exclude := make([]string, 0, len(node.Content))
	for _, pattern := range node.Content {
		if pattern.Kind != yaml.ScalarNode || pattern.Tag != yamlStringTag {
			return nil, ErrRecordCommand
		}

		exclude = append(exclude, pattern.Value)
	}

	return exclude, nil
}
