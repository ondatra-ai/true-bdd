package architecture

import (
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var errAIClisRetired = errors.New(
	"testing.testkit.ai_clis is retired; declare third_party CLI dependencies on the service")
var errFixtureStep = errors.New("testing.testkit.fixture_step must be a regular expression with one capture")

// UnmarshalYAML refuses the retired list even when its value is empty.
func (t *Testkit) UnmarshalYAML(node *yaml.Node) error {
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value == "ai_clis" {
			return errAIClisRetired
		}
	}

	type declaration Testkit

	var decoded declaration

	err := node.Decode(&decoded)
	if err != nil {
		return fmt.Errorf("decode testkit: %w", err)
	}

	if decoded.FixtureStep != "" {
		pattern, err := regexp.Compile(decoded.FixtureStep)
		if err != nil {
			return fmt.Errorf("%w: %w", errFixtureStep, err)
		}

		if pattern.NumSubexp() != 1 {
			return errFixtureStep
		}
	}

	*t = Testkit(decoded)

	return nil
}
