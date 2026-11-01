package steps

import "strings"

// answerSeparator divides the lines a `line by line` answer types.
const answerSeparator = "/"

// Each trimmed piece between separators is one stdin line, with nothing after the last.
//
//truebdd:concept given "scripted answers"
func answerLineByLine(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	var stdin strings.Builder

	for piece := range strings.SplitSeq(args[1], answerSeparator) {
		stdin.WriteString(strings.TrimSpace(piece))
		stdin.WriteString("\n")
	}

	state.Fixture.Stdin = []byte(stdin.String())

	return nil
}
