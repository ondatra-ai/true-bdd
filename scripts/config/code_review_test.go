package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/config"
)

// The pointer is the point: a plain bool zero-values to off, so a misspelled
// key would silently switch the step off instead of leaving it on.
func TestCodeReviewIsOnWhenTheKeyIsAbsent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")

	err := os.WriteFile(path, []byte(`{"postmortem": false}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	switches, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !config.On(switches.CodeReview) {
		t.Error("an absent code_review read as off; the step would vanish silently")
	}
}

func TestCodeReviewReadsBothSettings(t *testing.T) {
	t.Parallel()

	for body, want := range map[string]bool{
		`{"code_review": true}`:  true,
		`{"code_review": false}`: false,
	} {
		path := filepath.Join(t.TempDir(), "config.json")

		err := os.WriteFile(path, []byte(body), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		switches, loadErr := config.Load(path)
		if loadErr != nil {
			t.Fatal(loadErr)
		}

		if got := config.On(switches.CodeReview); got != want {
			t.Errorf("%s gave %v, want %v", body, got, want)
		}
	}
}

// The file this repository actually ships must parse and name the switch.
func TestTheRepositoryConfigDeclaresCodeReview(t *testing.T) {
	t.Parallel()

	switches, err := config.Load("../.config.json")
	if err != nil {
		t.Fatalf("scripts/.config.json does not parse: %v", err)
	}

	if switches.CodeReview == nil {
		t.Error("scripts/.config.json does not name code_review; the file should " +
			"read as the complete list of what this repo runs")
	}
}
