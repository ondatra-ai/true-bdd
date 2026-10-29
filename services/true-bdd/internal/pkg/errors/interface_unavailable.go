package errors

import (
	"errors"
	"fmt"
)

// ErrInterfaceCannotServe stops step authoring without another attempt.
var ErrInterfaceCannotServe = errors.New("public interface cannot serve phrase")

// InterfaceCannotServe names the phrase and the missing setup or observation.
func InterfaceCannotServe(phrase, lacks string) error {
	return fmt.Errorf("%w %q: %s; extend the Public interface declaration", ErrInterfaceCannotServe, phrase, lacks)
}
