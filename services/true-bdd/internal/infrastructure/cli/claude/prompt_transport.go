package claude

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/shell"
)

// Linux guarantees 128 KiB even at its minimum stack allowance; macOS permits
// at least 256 KiB. Reserve 16 KiB for executable/interpreter paths and PWD.
const argumentBudget = 128*1024 - 16*1024

// command freezes the child environment used to budget argv. A fitting request
// keeps its cassette's argv/stdin; otherwise -p reads the whole prompt at EOF.
func (o Options) command(prompt string) ([]string, shell.Options) {
	args := o.args()
	env := o.Env().Entries()
	options := shell.Options{Dir: o.Dir, Env: shell.Env{}.Exact(env), Timeout: o.Timeout}

	if argumentSize(args)+argumentSize(env)+len(Bin)+1+len(prompt)+1 <= argumentBudget {
		return append(args, prompt), options
	}

	options.Stdin = strings.NewReader(prompt)

	return args, options
}

// Count NUL-terminated strings and 64-bit pointers, including argv[0], the
// optional prompt pointer and both vector terminators. Each string then fits
// Linux's separate 128 KiB per-string limit as well as the total budget.
func argumentSize(entries []string) int {
	const (
		pointerBytes = 8
		extraPointers = 2
	)

	size := pointerBytes * (len(entries) + extraPointers)
	for _, entry := range entries {
		size += len(entry) + 1
	}

	return size
}
