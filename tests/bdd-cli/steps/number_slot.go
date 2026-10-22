package steps

import (
	"fmt"
	"strconv"
)

// numberSlot reads what a step's number slot holds: digits, or the word
// "no" for zero, as the catalogue's template grammar spells a count.
func numberSlot(token string) (int, error) {
	if token == "no" {
		return 0, nil
	}

	count, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("%q is not a count: %w", token, err)
	}

	return count, nil
}
