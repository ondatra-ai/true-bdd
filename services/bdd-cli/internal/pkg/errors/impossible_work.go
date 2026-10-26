package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrImpossibleWork distinguishes an architectural obstruction from non-convergence.
var ErrImpossibleWork = errors.New("repair is impossible within the architecture's declared writable paths")

// RequiredChange names an undeclared file and why no permitted solution exists.
type RequiredChange struct {
	Path   string `json:"path"   yaml:"path"`
	Reason string `json:"reason" yaml:"reason"`
}

// ImpossibleWork reports the model's first impossibility verdict, even after edits.
func ImpossibleWork(changes []RequiredChange) error {
	details := make([]string, 0, len(changes))
	for _, change := range changes {
		details = append(details, change.Path+": "+change.Reason)
	}

	return fmt.Errorf("%w: %s; change these files by hand, or add them to the service's writable",
		ErrImpossibleWork, strings.Join(details, "; "))
}
