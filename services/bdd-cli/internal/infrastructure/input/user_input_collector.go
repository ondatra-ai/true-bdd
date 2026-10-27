package input

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/console"
)

const separatorWidth = 60

// ErrPersonRequired means a clarification needs a person to answer.
var ErrPersonRequired = errors.New("a person must answer the clarifying question")

// UserInputCollector handles interactive user input from stdin.
type UserInputCollector struct {
	reader *bufio.Reader
}

// NewUserInputCollector creates a new UserInputCollector reading console.In().
func NewUserInputCollector() *UserInputCollector {
	return newUserInputCollector(console.In())
}

// newUserInputCollector builds a collector over an arbitrary reader so the
// line-based prompt parsing (choice enum, numeric-to-text clarify mapping,
// multiline freetext termination) is unit-testable without a real terminal.
func newUserInputCollector(reader io.Reader) *UserInputCollector {
	return &UserInputCollector{
		reader: bufio.NewReader(reader),
	}
}

// AskQuestions displays questions to the user and collects answers.
func (c *UserInputCollector) AskQuestions(questions []checklist.ClarifyQuestion) (map[string]string, error) {
	answers := make(map[string]string)

	c.printHeader()

	for idx, question := range questions {
		c.printQuestion(idx+1, len(questions), question)

		userInput, err := c.readUserInput()
		if errors.Is(err, io.EOF) {
			return nil, ErrPersonRequired
		}

		if err != nil {
			return nil, err
		}

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

	return answers, nil
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
func (c *UserInputCollector) AskApplyRefineOrExit() (ActionChoice, error) {
	console.Println("\n" + strings.Repeat("=", separatorWidth))
	console.Println("What would you like to do?")
	console.Separator("=", separatorWidth)
	console.Println("  [1] Apply this fix to working copy")
	console.Println("  [2] Refine the fix prompt (provide feedback)")
	console.Println("  [3] Exit without applying")
	console.Separator("=", separatorWidth)
	console.Print("Your choice (1/2/3): ")

	rawInput, err := c.readUserInput()
	if errors.Is(err, io.EOF) {
		return ActionExit, fmt.Errorf("end of input at fix menu; a person must choose whether to apply the fix: %w", err)
	}

	if err != nil {
		return ActionExit, err
	}

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

	return action, nil
}

// AskRefinementFeedback asks user for feedback to refine the fix prompt.
func (c *UserInputCollector) AskRefinementFeedback() (string, error) {
	console.Println("\n" + strings.Repeat("-", separatorWidth))
	console.Println("REFINE FIX PROMPT")
	console.Separator("-", separatorWidth)
	console.Println("Enter your feedback to improve the fix prompt.")
	console.Println("Press Enter twice when done.")
	console.BlankLine()

	var lines []string

	for {
		line, err := c.readUserInput()
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}

		if line == "" {
			break
		}

		lines = append(lines, line)
	}

	feedback := strings.Join(lines, "\n")

	slog.Info("User provided refinement feedback", "feedbackLength", len(feedback), "feedback", feedback)

	return feedback, nil
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

func (c *UserInputCollector) readUserInput() (string, error) {
	input, err := c.reader.ReadString('\n')
	if errors.Is(err, io.EOF) && len(input) > 0 {
		err = nil
	}

	if err != nil {
		return "", fmt.Errorf("read user input: %w", err)
	}

	return strings.TrimSpace(input), nil
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
