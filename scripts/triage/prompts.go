package triage

import _ "embed"

// Prompts are files, not string literals: literals can't carry backticks
// unescaped, and an accidental rewrite here would change what's asked
// without showing up as a change to a prompt.

// rubricPrompt scores one subject by consequence if left undone.
//
//go:embed rubric.txt
var rubricPrompt string

// refreshPrompt is appended for a subject whose body is a ticket to rewrite.
//
//go:embed refresh.txt
var refreshPrompt string

// storyPrompt is appended instead for a ticket with no `### Why` to write under.
//
//go:embed story.txt
var storyPrompt string
