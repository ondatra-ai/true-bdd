package errors

import (
	"errors"
	"fmt"
)

// ErrInterfaceCannotServe stops step authoring without another attempt.
var ErrInterfaceCannotServe = errors.New("public interface cannot serve")

// InterfaceCannotServe names the concept the writer's verdict names, as the
// Item's label gives it (`when concept "halving"`), and the missing setup or observation.
func InterfaceCannotServe(concept, lacks string) error {
	return fmt.Errorf("%w %s: %s; extend the Public interface declaration", ErrInterfaceCannotServe, concept, lacks)
}
