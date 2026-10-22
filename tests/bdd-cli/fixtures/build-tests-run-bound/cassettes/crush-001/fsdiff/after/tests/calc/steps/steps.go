package steps

import ("fmt"; "strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is (doubled|tripled|halved|squared|negated)$`, applyOperation)
 suite.Step(`^the result is (\d+)$`, result)
}

// operations is the action vocabulary the `^the number is (…)$` When
// definition serves. The regexp alternation above and the keys here are
// deliberately the same closed set: the pattern must stay a literal so
// the registry checker can read it, so adding a word means adding it in
// both places. The set excludes digits on purpose — a wildcard capture
// would also match the Given step `the number is 3` and leave that step
// matching two patterns.
var operations = map[string]func(int) (int, error){
	"doubled": func(v int) (int, error) { return v * 2, nil },
	"tripled": func(v int) (int, error) { return v * 3, nil },
	"squared": func(v int) (int, error) { return v * v, nil },
	"negated": func(v int) (int, error) { return -v, nil },
	"halved": func(v int) (int, error) {
		if v%2 != 0 {
			return 0, fmt.Errorf("cannot halve %d: expected an even number, got an odd one", v)
		}
		return v / 2, nil
	},
}

// applyOperation is the When action: it transforms the number currently
// in the state and writes the outcome back to state.Value, which is the
// field the `^the result is (\d+)$` Then definition asserts on.
func applyOperation(state *State, args []string) error {
	operation, ok := operations[args[0]]
	if !ok {
		return fmt.Errorf("expected a known operation on the number, got %q", args[0])
	}
	updated, err := operation(state.Value)
	if err != nil {
		return err
	}
	state.Value = updated
	return nil
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
