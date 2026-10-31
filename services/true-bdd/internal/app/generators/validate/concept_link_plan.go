package validate

import (
	"maps"
	"slices"
	"strconv"
)

// linkPlan is a walk's distinct steps in the fewest balanced batches of at most the shared batch size.
type linkPlan struct {
	batches [][]conceptStep
	batchOf map[string]int
}

func newLinkPlan(subjects []any, size int) linkPlan {
	unique := make(map[string]conceptStep)

	for _, subject := range subjects {
		for _, step := range conceptSteps(subject) {
			if !step.judged() {
				unique[step.identity()] = step
			}
		}
	}

	ordered := rekeyedSteps(unique)
	plan := linkPlan{batchOf: make(map[string]int)}
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
