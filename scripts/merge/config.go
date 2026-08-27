package merge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// configPath holds the switches this repository sets for its own tooling,
// relative because Start has already chdir'd to the repository root. JSON
// carries no comment, so each switch is documented where it is read.
const configPath = "scripts/.config.json"

// loadPostmortem answers whether this run files a postmortem. An absent file
// and an absent key both mean yes — what the merge did before the switch
// existed — because a bool's zero value would silently mean the opposite.
func loadPostmortem(path string) (bool, error) {
	//nolint:gosec // the path is a constant at every call site; the parameter is the test seam.
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	var config struct {
		Postmortem *bool `json:"postmortem"`
	}

	err = json.Unmarshal(raw, &config)
	if err != nil {
		return false, fmt.Errorf("parsing %s: %w", path, err)
	}

	if config.Postmortem == nil {
		return true, nil
	}

	return *config.Postmortem, nil
}
