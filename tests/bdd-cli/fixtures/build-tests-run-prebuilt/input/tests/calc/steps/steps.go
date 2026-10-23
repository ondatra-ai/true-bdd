package steps

import (
 "errors"

 "github.com/ondatra-ai/true-bdd/pkg/cli"
 "github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

func Register(suite Registrar) {
 suite.Step(`^the engine is available$`, available)
 suite.Step(`^the engine help is requested$`, help)
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
