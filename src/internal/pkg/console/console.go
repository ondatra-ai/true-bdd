// Package console provides terminal UI output functions.
// This package wraps fmt.Print* for user-facing CLI output (not logging).
// Use slog for application logging; use console for interactive terminal UI.
package console

import (
	"fmt"
	"strings"
)

// Print writes to stdout without a newline.
func Print(a ...any) {
	fmt.Print(a...)
}

// Println writes to stdout with a newline.
func Println(a ...any) {
	fmt.Println(a...)
}

// Printf writes formatted output to stdout.
func Printf(format string, a ...any) {
	fmt.Printf(format, a...)
}

// Separator prints a line of repeated characters.
func Separator(char string, width int) {
	fmt.Println(strings.Repeat(char, width))
}

// Header prints a header with separators above and below.
func Header(title string, width int) {
	Separator("=", width)
	Println(title)
	Separator("=", width)
}

// BlankLine prints an empty line.
func BlankLine() {
	fmt.Println()
}
