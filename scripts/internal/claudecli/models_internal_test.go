package claudecli

import "testing"

// The precedence is the whole point: a command beats its script, a script
// beats the default. Turns used to inherit the operator's user-scope model,
// so getting this order wrong silently re-tiers commit and merge turns.
func TestResolvePrefersTheMostSpecific(t *testing.T) {
	t.Parallel()

	const merge = "merge"

	config := models{
		Default:    "default-model",
		PerScript:  map[string]string{merge: "script-model"},
		PerCommand: map[string]string{"merge-fix": "command-model"},
	}

	tests := []struct {
		name    string
		script  string
		command string
		want    string
	}{
		{"command wins over script", merge, "merge-fix", "command-model"},
		{"script wins over default", merge, "commit-msg", "script-model"},
		{"default catches the rest", "commit", "commit-msg", "default-model"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := resolve(config, testCase.script, testCase.command); got != testCase.want {
				t.Errorf("resolve(%q, %q) = %q, want %q",
					testCase.script, testCase.command, got, testCase.want)
			}
		})
	}
}

// An unconfigured repo leaves every turn on whatever its call site named.
func TestResolveEmptyConfigNamesNothing(t *testing.T) {
	t.Parallel()

	if got := resolve(models{}, "commit", "commit-msg"); got != "" {
		t.Errorf("resolve on empty config = %q, want empty", got)
	}
}

// The shipped config and this loader have to agree: a renamed key leaves every
// turn silently on the CLI's default. Read from the repo root, which is where
// every scripts/ binary runs.
func TestShippedConfigCarriesADefault(t *testing.T) {
	// No t.Parallel: t.Chdir forbids it.
	t.Chdir("../../..")

	if got := load().Default; got == "" {
		t.Errorf("%s names no scripts.models.default", configPath)
	}
}
