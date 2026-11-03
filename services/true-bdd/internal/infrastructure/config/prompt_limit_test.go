package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
)

func TestHostPromptByteLimit(t *testing.T) {
	const defaultLimit = 150000
	for _, test := range []struct {
		setting string
		want    int
		invalid bool
	}{
		{"{}", defaultLimit, false}, {"engine: {max_prompt_bytes: 80000}", 80000, false},
		{"engine: {max_prompt_bytes: 0}", 0, true}, {"engine: {max_prompt_bytes: -1}", 0, true},
		{"engine: {max_prompt_bytes: 1.5}", 0, true}, {"engine: {max_prompt_bytes: invalid}", 0, true},
	} {
		t.Run(test.setting, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)

			err := os.Mkdir(filepath.Join(root, "true-bdd"), 0750)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile("true-bdd/true-bdd.yaml", []byte(test.setting), 0600)
			if err != nil {
				t.Fatal(err)
			}

			cfg, err := config.NewViperConfig()
			if err != nil {
				t.Fatal(err)
			}

			got, err := cfg.PromptLimitBytes(defaultLimit)
			if got != test.want || (err != nil) != test.invalid {
				t.Fatalf("got %d, %v", got, err)
			}
		})
	}
}
