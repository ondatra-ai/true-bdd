// Package config is scripts/.config.json, the switches this repository sets
// for its own tooling. Read once at the start of a run, so a step that is
// switched off costs nothing and says so where it would have run.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Path is relative because every scripts/ entrypoint chdirs to the repository
// root before it does anything else.
const Path = "scripts/.config.json"

// Switches is the file. A nil field is unset, which On reads as on: a plain
// bool zero-values to off, so an absent or misspelled key would silently
// disable the step it names.
type Switches struct {
	Postmortem   *bool `json:"postmortem"`
	DocUniverse  *bool `json:"doc_universe"`
	UpdateMemory *bool `json:"update_memory"`
}

// On is what an unset switch means — what the tooling did before the switch
// existed.
func On(flag *bool) bool { return flag == nil || *flag }

// Load reads the switches. An absent file leaves every one of them unset; a
// file that does not parse is a stop, not a default.
func Load(path string) (Switches, error) {
	var switches Switches

	//nolint:gosec // the path is a constant at every call site; the parameter is the test seam.
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return switches, nil
	}

	if err != nil {
		return switches, fmt.Errorf("reading %s: %w", path, err)
	}

	err = json.Unmarshal(raw, &switches)
	if err != nil {
		return switches, fmt.Errorf("parsing %s: %w", path, err)
	}

	return switches, nil
}
