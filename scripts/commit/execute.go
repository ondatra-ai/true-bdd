package commit

import (
	"errors"
	"strings"
)

// GatesRedPrefix opens the stop a red gate produces. Exported because it is the
// one stop a caller may retry, and matching it on a copied string literal is how
// that classification silently stops being true.
const GatesRedPrefix = "the gates are red — nothing was committed:"

// stopSentinel is what dief and usage unwind with. Unexported: nothing outside
// this package can forge a stop.
type stopSentinel struct{ message string }

// StopError is a run that stopped itself, so an importer can tell a refused run
// from a crash.
type StopError struct{ Message string }

func (e *StopError) Error() string { return e.Message }

// IsGatesRed reports whether err is the one stop a caller may recover from.
func IsGatesRed(err error) bool {
	var stopped *StopError

	return errors.As(err, &stopped) && strings.HasPrefix(stopped.Message, GatesRedPrefix)
}

// Execute runs the whole workflow and returns what stopped it.
func Execute(args []string) error {
	return guard(func() { Start(args).Main() })
}

// Embed runs it inside a parent that imported this package: no report render,
// because the parent folds the whole tree out of the same log.
func Embed() error {
	return guard(func() {
		run := Start(nil)
		run.embedded = true

		run.Main()
	})
}

// guard turns a stop back into an error. Anything else is re-panicked: a
// genuine crash must not become a polite error.
// The named result is what the deferred recover assigns through.
func guard(body func()) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		stopped, ok := recovered.(stopSentinel)
		if !ok {
			panic(recovered)
		}

		err = &StopError{Message: stopped.message}
	}()

	body()

	return nil
}
