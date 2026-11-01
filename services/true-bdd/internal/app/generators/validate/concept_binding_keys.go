package validate

import (
	"maps"
	"slices"
)

func sortedBindingKeys(linked map[string]linkedBinding) []string {
	return slices.Sorted(maps.Keys(linked))
}
