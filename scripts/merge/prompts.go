package merge

import _ "embed"

// The prompts are files, not string literals, for two reasons: they are dense
// with backticks that a Go literal cannot carry unescaped, and a prompt this
// package rewrote by accident would change what a model is asked without
// showing up as a change to a prompt.

// rubricPrompt scores each finding by consequence if left unfixed.
//
//go:embed prompts/rubric.txt
var rubricPrompt string

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
