package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/config"
)

// fixtureMode is what a config file is written with.
const fixtureMode = 0o600

// TestLoadSwitches pins the two ways a switch can be unset — no file and no
// key — reading the same way, which a plain bool would not do.
func TestLoadSwitches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		body         string // "" writes no file at all
		postmortem   bool
		docUniverse  bool
		updateMemory bool
		wantErr      bool
	}{
		{name: "no file", postmortem: true, docUniverse: true, updateMemory: true},
		{name: "no key", body: `{}`, postmortem: true, docUniverse: true, updateMemory: true},
		{name: "one off", body: `{"postmortem": false}`, docUniverse: true, updateMemory: true},
		{name: "all off", body: `{"postmortem": false, "doc_universe": false, "update_memory": false}`},
		{
			name: "all on", body: `{"postmortem": true, "doc_universe": true, "update_memory": true}`,
			postmortem: true, docUniverse: true, updateMemory: true,
		},
		{name: "malformed", body: `{"postmortem":`, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			switches, err := config.Load(writeFixture(t, testCase.body))
			if (err != nil) != testCase.wantErr {
				t.Fatalf("error %v, wanted an error: %v", err, testCase.wantErr)
			}

			if err != nil {
				return
			}

			assertSwitch(t, "postmortem", config.On(switches.Postmortem), testCase.postmortem)
			assertSwitch(t, "doc_universe", config.On(switches.DocUniverse), testCase.docUniverse)
			assertSwitch(t, "update_memory", config.On(switches.UpdateMemory), testCase.updateMemory)
		})
	}
}

// writeFixture answers a path, which an empty body leaves with no file on it.
func writeFixture(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".config.json")

	if body == "" {
		return path
	}

	err := os.WriteFile(path, []byte(body), fixtureMode)
	if err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	return path
}

func assertSwitch(t *testing.T, name string, got, want bool) {
	t.Helper()

	if got != want {
		t.Errorf("%s %v, want %v", name, got, want)
	}
}
