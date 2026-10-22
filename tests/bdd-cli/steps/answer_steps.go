package steps

import "strings"

// answerSeparator divides the lines a `line by line` answer types.
const answerSeparator = "/"

// assertAnswersLineByLine sets the fixture's stdin to the given lines, in
// turn and nothing after them: each piece between separators, trimmed, is one
// line, so an empty piece is the empty line that ends refinement feedback.
func assertAnswersLineByLine(state *State, args []string) error {
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
