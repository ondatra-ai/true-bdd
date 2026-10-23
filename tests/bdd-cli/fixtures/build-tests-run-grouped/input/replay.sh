#!/bin/sh
set -eu
if [ ! -f tmp/injected ]; then
  mkdir -p tmp
  touch tmp/injected
  cat > tests/calc/steps/steps.go <<'BROKEN'
package steps

import ("fmt"; "strconv"; "strings")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is doubled$`, func(s *State, _ []string) error { s.Value *= 3; return nil })
 suite.Step(`^the result is (\d+)$`, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; if value == 3 { state.Value++ }; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d\nEVIDENCE-BEGIN\n%sEVIDENCE-END", want, state.Value, strings.Repeat("full runtime evidence\n", 600)) }; return nil
}
BROKEN
fi
exec go test -tags bdd -count=1 ./tests/calc/ "$@"
