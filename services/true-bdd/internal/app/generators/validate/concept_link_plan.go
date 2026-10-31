package validate

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// linkPlan is a walk's distinct steps in the fewest balanced batches of at most the shared batch size, each
// step with the descriptions of the walked Scenarios using it.
type linkPlan struct {
	batches      [][]conceptStep
	batchOf      map[string]int
	descriptions map[string][]string
}

// sharedLinkStep is one step of a shared link turn: its block and text, and the descriptions of its Scenarios.
type sharedLinkStep struct {
	Key          string          `json:"key"`
	Block        catalogue.Block `json:"block"`
	Text         string          `json:"text"`
	Descriptions []string        `json:"descriptions"`
}

func newLinkPlan(subjects []any, size int) linkPlan {
	unique := make(map[string]conceptStep)
	plan := linkPlan{batchOf: make(map[string]int), descriptions: make(map[string][]string)}

	for _, subject := range subjects {
		description := strings.TrimSpace(conceptItemTitle(subject))

		for _, step := range conceptSteps(subject) {
			if step.judged() {
				continue
			}

			unique[step.identity()] = step

			known := plan.descriptions[step.identity()]
			if description != "" && !slices.Contains(known, description) {
				plan.descriptions[step.identity()] = append(known, description)
			}
		}
	}

	for _, known := range plan.descriptions {
		slices.Sort(known)
	}

	ordered := rekeyedSteps(unique)
	count := (len(ordered) + max(1, size) - 1) / max(1, size)

	for number := range count {
		batch := rekey(ordered[number*len(ordered)/count : (number+1)*len(ordered)/count])
		for _, step := range batch {
			plan.batchOf[step.identity()] = number
		}

		plan.batches = append(plan.batches, batch)
	}

	return plan
}

// batchSteps is a batch as its shared turn reads it.
func (plan linkPlan) batchSteps(number int) []sharedLinkStep {
	steps := make([]sharedLinkStep, 0, len(plan.batches[number]))
	for _, step := range plan.batches[number] {
		descriptions := plan.descriptions[step.identity()]
		if descriptions == nil {
			descriptions = []string{}
		}

		steps = append(steps, sharedLinkStep{Key: step.Key, Block: step.Block, Text: step.Text, Descriptions: descriptions})
	}

	return steps
}

// batchesFor names the batches holding the steps a link judges, or false when one is outside the plan.
func (plan linkPlan) batchesFor(steps []conceptStep) ([]int, bool) {
	numbers := make([]int, 0)

	for _, step := range steps {
		if step.judged() {
			continue
		}

		number, planned := plan.batchOf[step.identity()]
		if !planned {
			return nil, false
		}

		if !slices.Contains(numbers, number) {
			numbers = append(numbers, number)
		}
	}

	slices.Sort(numbers)

	return numbers, true
}

// rekeyedSteps orders distinct steps by block and text and keys them from 1.
func rekeyedSteps(unique map[string]conceptStep) []conceptStep {
	ordered := make([]conceptStep, 0, len(unique))
	for _, identity := range slices.Sorted(maps.Keys(unique)) {
		ordered = append(ordered, unique[identity])
	}

	return rekey(ordered)
}

func rekey(steps []conceptStep) []conceptStep {
	keyed := make([]conceptStep, 0, len(steps))
	for index, step := range steps {
		keyed = append(keyed, conceptStep{Key: strconv.Itoa(index + 1), Block: step.Block, Text: step.Text})
	}

	return keyed
}
