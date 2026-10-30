package validate

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

const schemaNull = "null"
const schemaInteger = "integer"
const schemaType = "type"
const schemaString = "string"
const schemaMinLength = "minLength"
const schemaReason = "reason"
const maxConceptScore = 10
const defaultLinkThreshold = 9

type answerSchema map[string]any

func requiredObject(properties answerSchema) answerSchema {
	return answerSchema{schemaType: "object", "properties": properties,
		"required": slices.Sorted(maps.Keys(properties)), "additionalProperties": false}
}

func linkSchema(steps []conceptStep, concepts []product.Concept) (string, error) {
	properties := answerSchema{}

	for _, step := range steps {
		if step.judged() {
			continue
		}

		scores := answerSchema{}

		for _, concept := range concepts {
			if concept.Block == step.Block {
				scores[concept.Name] = answerSchema{schemaType: schemaInteger, "minimum": 0, "maximum": maxConceptScore}
			}
		}

		properties[step.Key] = requiredObject(answerSchema{
			"scores": requiredObject(scores), schemaReason: answerSchema{schemaType: schemaString, schemaMinLength: 1},
			"other_block_concept": answerSchema{schemaType: []string{schemaString, schemaNull}},
		})
	}

	encoded, err := json.Marshal(requiredObject(answerSchema{"steps": requiredObject(properties)}))
	if err != nil {
		return "", fmt.Errorf("encode link schema: %w", err)
	}

	return string(encoded), nil
}
