package input

import (
	"bufio"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/events"
)

const separatorWidth = 60

// UserInputCollector handles interactive user input from stdin.
type UserInputCollector struct {
	reader  *bufio.Reader
	emitter *events.Emitter
}

// NewUserInputCollector creates a new UserInputCollector reading console.In().
// The emitter is a no-op unless TRUE_BDD_EVENTS_FILE is set (under
// `true-bdd remote`), so default CLI behavior is unchanged.
func NewUserInputCollector() *UserInputCollector {
	return newUserInputCollector(console.In())
}

// NewUserInputCollectorFrom builds a collector over an arbitrary reader. The
// hidden `prompt-probe` command (plan §4) uses it to drive the real prompt
// dialogs against controlled stdin without spawning `claude`.
func NewUserInputCollectorFrom(reader io.Reader) *UserInputCollector {
	return newUserInputCollector(reader)
}

// newUserInputCollector builds a collector over an arbitrary reader so the
// line-based prompt parsing (choice enum, numeric-to-text clarify mapping,
// multiline freetext termination) is unit-testable without a real terminal.
func newUserInputCollector(reader io.Reader) *UserInputCollector {
	return &UserInputCollector{
		reader:  bufio.NewReader(reader),
		emitter: events.NewEmitter(),
	}
}

// AskQuestions displays questions to the user and collects answers.
func (c *UserInputCollector) AskQuestions(questions []checklist.ClarifyQuestion) map[string]string {
	answers := make(map[string]string)

	c.printHeader()

	for idx, question := range questions {
		c.printQuestion(idx+1, len(questions), question)

		// Publish the clarify prompt on the event channel immediately
		// before blocking on stdin (plan §3.2). No-op without the env var.
		c.emitter.EmitClarifyPrompt(question.Question, question.Options)

		userInput := c.readUserInput()
		userInput = c.mapOptionToText(userInput, question.Options)

		slog.Info("User answered clarification question",
			"questionID", question.ID,
			"question", question.Question,
			"answer", userInput,
		)

		answers[question.ID] = userInput

		console.Printf("    Recorded: %s\n", userInput)
	}

	console.Println("\n" + strings.Repeat("-", separatorWidth))

	return answers
}

// ActionChoice represents user's choice for fix prompt action.
type ActionChoice string

const (
	// ActionApply indicates user wants to apply the fix.
	ActionApply ActionChoice = "apply"
	// ActionRefine indicates user wants to refine the fix prompt.
	ActionRefine ActionChoice = "refine"
	// ActionExit indicates user wants to exit without applying.
	ActionExit ActionChoice = "exit"
)

// AskApplyRefineOrExit asks user to choose between apply, refine, or exit.
func (c *UserInputCollector) AskApplyRefineOrExit() ActionChoice {
	console.Println("\n" + strings.Repeat("=", separatorWidth))
	console.Println("What would you like to do?")
	console.Separator("=", separatorWidth)
	console.Println("  [1] Apply this fix to working copy")
	console.Println("  [2] Refine the fix prompt (provide feedback)")
	console.Println("  [3] Exit without applying")
	console.Separator("=", separatorWidth)
	console.Print("Your choice (1/2/3): ")

	// Publish the choice prompt on the event channel immediately before
	// blocking on stdin (plan §3.2). No-op without the env var.
	c.emitter.EmitChoicePrompt([]string{
		string(ActionApply), string(ActionRefine), string(ActionExit),
	})

	rawInput := c.readUserInput()

	var action ActionChoice

	switch rawInput {
	case "1", "apply":
		action = ActionApply
	case "2", "refine":
		action = ActionRefine
	default:
		action = ActionExit
	}

	slog.Info("User chose fix action", "rawInput", rawInput, "action", string(action))

	return action
}

// AskRefinementFeedback asks user for feedback to refine the fix prompt.
func (c *UserInputCollector) AskRefinementFeedback() string {
	console.Println("\n" + strings.Repeat("-", separatorWidth))
	console.Println("REFINE FIX PROMPT")
	console.Separator("-", separatorWidth)
	console.Println("Enter your feedback to improve the fix prompt.")
	console.Println("Press Enter twice when done.")
	console.BlankLine()

	// Publish the freetext prompt on the event channel immediately
	// before reading the multiline block (plan §3.2). No-op without env.
	c.emitter.EmitFreetextPrompt("Enter your feedback to improve the fix prompt.")

	var lines []string

	for {
		line := c.readUserInput()
		if line == "" {
			break
		}

		lines = append(lines, line)
	}

	feedback := strings.Join(lines, "\n")

	slog.Info("User provided refinement feedback", "feedbackLength", len(feedback), "feedback", feedback)

	return feedback
}

// Private methods (must come after all exported methods per funcorder lint rule)

func (c *UserInputCollector) printHeader() {
	console.Println("\n" + strings.Repeat("-", separatorWidth))
	console.Println("CLARIFICATION NEEDED")
	console.Separator("-", separatorWidth)
}

func (c *UserInputCollector) printQuestion(current, total int, question checklist.ClarifyQuestion) {
	console.Printf("\n[%d/%d] %s\n", current, total, question.Question)

	if question.Context != "" {
		console.Printf("    Context: %s\n", question.Context)
	}

	if len(question.Options) > 0 {
		console.Println("    Options:")

		for idx, opt := range question.Options {
			console.Printf("      %d) %s\n", idx+1, opt)
		}

		console.Println("      0) Other (type custom answer)")
	}

	console.Print("\n    Your answer: ")
}

func (c *UserInputCollector) readUserInput() string {
	input, _ := c.reader.ReadString('\n')

	return strings.TrimSpace(input)
}

func (c *UserInputCollector) mapOptionToText(input string, options []string) string {
	if len(options) == 0 {
		return input
	}

	num, err := strconv.Atoi(input)
	if err != nil {
		return input
	}

	if num > 0 && num <= len(options) {
		return options[num-1]
	}

	return input
}
