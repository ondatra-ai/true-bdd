package taskhandle

import _ "embed"

// The prompts are files, not literals, so a wording change is a diff on prose
// rather than on Go.

//go:embed prompts/plan.txt
var planPrompt string

//go:embed prompts/implement.txt
var implementPrompt string

//go:embed prompts/fix.txt
var fixPrompt string

//go:embed prompts/narrow.txt
var narrowPrompt string

//go:embed prompts/review.txt
var reviewPrompt string
