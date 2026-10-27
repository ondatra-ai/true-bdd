package steps

import (
 "errors"
 "fmt"
 "strings"

 "github.com/ondatra-ai/true-bdd/pkg/cli"
 "github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

func Register(suite Registrar) {
 suite.Step(`^the engine is available$`, available)
 suite.Step(`^the engine help is requested$`, help)
 suite.Step(`^the help lists (.+)$`, helpLists)
}
func available(state *State, _ []string) error {
 if state.EngineBin == "" { return errors.New("no prebuilt engine supplied") }; return nil
}
func help(state *State, _ []string) error {
 result, err := spec.Run([]string{state.EngineBin, "--help"}, cli.Options{})
 if err != nil { return err }
 state.Help = result.Stdout
 return result.Err()
}

// helpLists asserts the help output captured by `help` advertises the
// command named in the step text.
func helpLists(state *State, args []string) error {
 if len(args) == 0 {
  return errors.New(`the help lists <command>: step matched but captured no command name`)
 }
 // The pattern has exactly one capture group, so its value is the last
 // element whether or not the harness prepends the full match.
 command := strings.TrimSpace(args[len(args)-1])
 if command == "" {
  return errors.New(`the help lists <command>: captured command name is empty`)
 }
 if state.Help == "" {
  return fmt.Errorf("expected the engine help to list %q, but no help output was captured: request the engine help before asserting on it", command)
 }
 if !strings.Contains(state.Help, command) {
  return fmt.Errorf("expected the engine help to list %q, but it is absent from the help output; got:\n%s", command, state.Help)
 }
 return nil
}
