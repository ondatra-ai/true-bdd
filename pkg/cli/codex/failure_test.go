package codex_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
)

func TestTerminalStatus(t *testing.T) {
	for _, test := range []struct{ name, output, want string }{
		{name: "workspace-spend-cap",
			output: "ERROR: You hit your spend cap set in your workspace. Increase your spend cap to continue.",
			want:   "spend-cap"},
		{name: "spend-cap-in-log",
			output: "session id: session-237\n" +
				"ERROR: You hit your spend cap set in your workspace. Increase your spend cap to continue.\n",
			want:   "spend-cap"},
		{name: "usage-limit", output: "ERROR: You've hit your usage limit.", want: "limit"},
		{name: "usage-limit-with-window", output: "ERROR: You’ve hit your usage limit for this week.", want: "limit"},
		{name: "expired-token", output: "ERROR: Your access token could not be refreshed.", want: "signed-out"},
		{name: "expired-token-with-cause",
			output: "ERROR: Your access token could not be refreshed because it has expired.", want: "signed-out"},
		{name: "ordinary-failure", output: "ERROR: Something else went wrong."},
		{name: "quoted-spend-cap", output: "The log showed ERROR: You hit your spend cap set in your workspace."},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := codex.TerminalStatus(test.output); got != test.want {
				t.Fatalf("TerminalStatus(%q) = %q, want %q", test.output, got, test.want)
			}
		})
	}
}
