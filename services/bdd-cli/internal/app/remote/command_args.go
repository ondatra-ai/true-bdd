package remote

import "fmt"

// Non-story command names (plan §3.1/§4). Named so the mapping, the executor
// classification, and the dispatch tests share one source of truth.
const (
	commandVersion     = "version"
	commandPromptProbe = "prompt-probe"
)

// commandArgs maps a dispatched run's typed command to the CLI argument
// vector the remote spawns its own binary with (plan §3.2). The store's
// enum allowlist has already validated the command; an unknown value
// here is defensive.
func commandArgs(run RunSpec) ([]string, error) {
	switch run.Command {
	case commandVersion:
		return []string{commandVersion}, nil
	case commandPromptProbe:
		return []string{commandPromptProbe}, nil
	case "us-create":
		return storyArgs("create", run)
	case "us-refine":
		return storyArgs("refine", run)
	case "us-apply":
		return storyArgs("apply", run)
	case "build-tests":
		return withFix([]string{"build", "tests"}, run.Fix), nil
	case "build-code":
		return withFix([]string{"build", "code"}, run.Fix), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownCommand, run.Command)
	}
}

// storyArgs builds `us <verb> <story-id> [--fix]`.
func storyArgs(verb string, run RunSpec) ([]string, error) {
	if run.StoryID == "" {
		return nil, fmt.Errorf("%w: %s", errNoStoryID, run.Command)
	}

	return withFix([]string{"us", verb, run.StoryID}, run.Fix), nil
}

// withFix appends --fix when requested.
func withFix(args []string, fix bool) []string {
	if fix {
		return append(args, "--fix")
	}

	return args
}
