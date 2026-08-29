package merge

import _ "embed"

// Prompts are files, not string literals: literals can't carry backticks
// unescaped, and an accidental rewrite here would change what's asked
// without showing up as a change to a prompt.

// fixPrompt asks for exactly one finding to be fixed, gates left green.
//
//go:embed prompts/fix.txt
var fixPrompt string

// postmortemPrompt diagnoses the run, not the code it merged.
//
//go:embed prompts/postmortem.txt
var postmortemPrompt string

// commitPrompt writes the message for a round's fix commit.
//
//go:embed prompts/commit.txt
var commitPrompt string
