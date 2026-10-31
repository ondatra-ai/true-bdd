package errors

import (
	"errors"
	"fmt"
)

// ErrInterfaceCannotServe stops step authoring without another attempt.
var ErrInterfaceCannotServe = errors.New("public interface cannot serve")

// InterfaceCannotServe names the concept Item the writer's verdict names
// and the missing setup or observation.
func InterfaceCannotServe(item, lacks string) error {
	return fmt.Errorf("%w %q: %s; extend the Public interface declaration", ErrInterfaceCannotServe, item, lacks)
}
