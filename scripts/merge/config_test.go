package merge_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

// fixtureMode is what a config file is written with.
const fixtureMode = 0o600

// TestLoadPostmortem pins the two ways the switch can be unset — no file and
// no key — defaulting the same way, which a plain bool would not do.
func TestLoadPostmortem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string // "" writes no file at all
		want    bool
		wantErr bool
	}{
		{name: "no file", body: "", want: true},
		{name: "switched off", body: `{"postmortem": false}`, want: false},
		{name: "switched on", body: `{"postmortem": true}`, want: true},
		{name: "key absent", body: `{}`, want: true},
		{name: "malformed", body: `{"postmortem":`, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), ".config.json")

			if testCase.body != "" {
				err := os.WriteFile(path, []byte(testCase.body), fixtureMode)
				if err != nil {
					t.Fatalf("writing the fixture: %v", err)
				}
			}

			got, err := merge.LoadPostmortem(path)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("error %v, wanted an error: %v", err, testCase.wantErr)
			}

			if err == nil && got != testCase.want {
				t.Errorf("postmortem %v, want %v", got, testCase.want)
			}
		})
	}
}
