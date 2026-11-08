package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var errNoStandIn = errors.New("no CLI stand-in is active for this run")

// StandIn locates the named stand-in installed for a run, for a fixture launcher that delegates to it.
func StandIn(run interface{ Environment() []string }, name string) (string, error) {
	directory := ""

	for _, entry := range run.Environment() {
		if value, ok := strings.CutPrefix(entry, "TRUE_BDD_AIPROXY_DIR="); ok {
			directory = value
		}
	}

	if directory == "" {
		return "", errNoStandIn
	}

	return filepath.Join(directory, name), nil
}

// Consumed lists a replaying dependency's recording names in consumption order.
func Consumed(run interface{ ObservationDirectory() string }, binary string) ([]string, error) {
	data, err := disk.Read(filepath.Join(run.ObservationDirectory(), "consumed-"+binary))
	if err != nil {
		return nil, fmt.Errorf("read consumed recordings: %w", err)
	}

	var names []string

	err = json.Unmarshal(data, &names)
	if err != nil {
		return nil, fmt.Errorf("decode consumed recordings: %w", err)
	}

	return names, nil
}
