package testrunner_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestCanonicalReplayCommand(t *testing.T) {
	t.Parallel()

	cases := []struct{ framework, command, want, selector string }{
		{"go-test", "go   test -json ./... -run \"^TestA$|Test B\"", "go test ./... -run '^TestA$|Test B'", "-run"},
		{"jest", "npx jest --json --config jest.config.js", "npx jest --config jest.config.js", "--testNamePattern"},
		{"playwright", "npx playwright test --reporter=json", "npx playwright test", "--grep"},
		{"playwright", "npx playwright test --reporter line,json", "npx playwright test --reporter line", "--grep"},
		{"go-test", "go test --json -args \"\" '$HOME' 'a;b' '*.go'", "go test -args '' '$HOME' 'a;b' '*.go'", "-run"},
	}
	for _, tc := range cases {
		got, selector, err := testrunner.CanonicalReplayCommand(tc.framework, tc.command)
		if err != nil || got != tc.want || selector != tc.selector {
			t.Errorf("%q: %q %q %v", tc.command, got, selector, err)
		}
	}

	for _, command := range []string{"go test -json \"a'b\"", "go test -json 'a\nb'", "go test -json 'a\x00b'"} {
		got, _, err := testrunner.CanonicalReplayCommand("go-test", command)
		if err == nil || got != "" {
			t.Errorf("unrepresentable %q: %q %v", command, got, err)
		}
	}
}
