package runner

import "strings"

// runDirToken in a scenario's command stands for the run dir's absolute
// path: the same token a cassette stores the run dir as.
const runDirToken = "{{CWD}}"

// commandArgs splits the scenario's command at whitespace, with no
// quoting, and names the run dir wherever the command says runDirToken.
func commandArgs(cmd, runDir string) []string {
	args := strings.Fields(cmd)
	for i, arg := range args {
		args[i] = strings.ReplaceAll(arg, runDirToken, runDir)
	}

	return args
}
