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
 suite.Step(`^the help lists "?([^"]+)"?$`, helpLists)
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

// helpLists asserts that the help output captured by the help step lists the
// command named in the step text.
func helpLists(state *State, args []string) error {
	if len(args) == 0 {
		return errors.New(`step "the help lists ...": expected one captured command name, got no captures`)
	}
	// The suite hands over the pattern's capture groups; reading the last
	// element yields this pattern's single group whether or not the full
	// match is prepended.
	want := strings.TrimSpace(args[len(args)-1])
	if want == "" {
		return errors.New(`step "the help lists ...": expected a non-empty command name, got an empty capture`)
	}
	if state.Help == "" {
		return fmt.Errorf("expected the engine help output to list %q, got no help output at all: the help request step did not run or produced nothing on stdout", want)
	}
	if !strings.Contains(state.Help, want) {
		return fmt.Errorf("expected the engine help output to list %q, got help output without it:\n%s", want, state.Help)
	}
	return nil
}
