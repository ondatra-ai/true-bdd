package commit

import _ "embed"

// The prompts, kept as files so a wording change is a diff on prose rather
// than on Go string literals.
var (
	//go:embed prompts/doc-universe.txt
	docUniversePrompt string
	//go:embed prompts/memory.txt
	memoryPrompt string
	//go:embed prompts/branch.txt
	branchPrompt string
	//go:embed prompts/commit.txt
	commitPrompt string
	//go:embed prompts/pr.txt
	pullRequestPrompt string
)
