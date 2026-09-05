package steps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotAStatusSet is returned when a step's status tail is neither a number
// nor numbers joined by " or ".
var ErrNotAStatusSet = errors.New("not a status, nor statuses joined by \" or \"")

// statusSet is what a clause accepts. More than one is written when the
// outcome is legitimately either — a frozen CLI's dispatch times out, unless
// the relay has already dropped the session, and both are the same fact.
type statusSet []int

// parseStatusSet reads the "504" or "504 or 404" tail of a step.
func parseStatusSet(raw string) (statusSet, error) {
	parts := strings.Split(raw, " or ")
	set := make(statusSet, 0, len(parts))

	for _, part := range parts {
		code, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("%w: %q", ErrNotAStatusSet, raw)
		}

		set = append(set, code)
	}

	return set, nil
}

// String renders the set as the step wrote it, so a failure quotes what it was
// asked for rather than a Go slice.
func (set statusSet) String() string {
	codes := make([]string, 0, len(set))

	for _, code := range set {
		codes = append(codes, strconv.Itoa(code))
	}

	return strings.Join(codes, " or ")
}

// holds answers whether a status is one the step named.
func (set statusSet) holds(status int) bool {
	for _, code := range set {
		if code == status {
			return true
		}
	}

	return false
}
