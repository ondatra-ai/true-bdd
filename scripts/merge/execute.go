package merge

// stopSentinel is what dief and usage unwind with. Unexported: nothing outside
// this package can forge a stop.
type stopSentinel struct{ message string }

// StopError is a run that stopped itself, so an importer can tell a refused run
// from a crash. No classifier accompanies it: per task-handle's spec no merge
// failure is recoverable, so every one of them halts.
type StopError struct{ Message string }

func (e *StopError) Error() string { return e.Message }

// Execute merges the current branch's PR and returns what stopped it.
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
