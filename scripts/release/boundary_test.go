package release_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

const (
	dispatchMessage   = "Dispatching AI turn"
	aiBoundary        = "AI boundary"
	configurationStep = "configuration"
)

func TestZeroCostPassRequiresDispatchSentinelAndFailure(t *testing.T) {
	cases := []struct{ name, old, replacement, step string }{
		{"no dispatch", dispatchMessage, `Running tests`, "suite discovery"},
		{"no sentinel", "\nclaude\n", "\ntrue\n", aiBoundary},
		{"successful exit", "\nexit 1\n", "\nexit 0\n", aiBoundary},
		{configurationStep, dispatchMessage, `Refusing to start`, configurationStep},
		{"Specification", dispatchMessage, `Loaded architecture`, "Specification"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := sourceRepository(t)
			engine := strings.ReplaceAll(installedEngine, testCase.old, testCase.replacement)
			artifacts := artifactSet(t, engine)
			report, err := release.Rehearse(context.Background(), root, artifacts, release.Options{})

			t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

			if err == nil || report.Passed {
				t.Fatalf("premature exit accepted: %+v", report)
			}

			if !strings.Contains(err.Error(), testCase.step) {
				t.Fatalf("wrong failing step: %v", err)
			}
		})
	}
}

func TestReportPreservesPerTurnDenialsAndReportedSpend(t *testing.T) {
	root := sourceRepository(t)

	const denial = `{"msg":"Tool calls were denied; this turn did less than it was asked to",` +
		`"denied":"Read /blocked/source.go; Glob /blocked/*.go", "label":"claude-turn001", "role":"fix_generator"}`

	records := "echo '" + denial + "' >> tmp/true-bdd.log.json\n" +
		"echo '{\"msg\":\"AI turn usage\",\"cli\":\"claude\",\"cost_usd\":1.25}' >> tmp/true-bdd.log.json\n" +
		"claude\nexit 1"

	engine := strings.Replace(installedEngine, "claude\nexit 1", records, 1)
	artifacts := artifactSet(t, engine)
	report, err := release.Rehearse(context.Background(), root, artifacts, release.Options{})

	t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

	if err != nil {
		t.Fatal(err)
	}

	host := report.Hosts[0]
	if host.ClaudeSpend != "$1.250000" || host.CodexSpend != "unavailable" {
		t.Fatalf("incorrect spend: %+v", host)
	}

	if len(host.Turns) != 1 || len(host.Turns[0].Denials) != 1 {
		t.Fatalf("denials lost: %+v", host)
	}

	if host.DenialTargets == "unavailable" {
		t.Fatal("#219 target evidence marked unavailable")
	}

	if !strings.Contains(string(host.Turns[0].Denials[0]), "/blocked/source.go") {
		t.Fatalf("target lost: %+v", host.Turns)
	}
}
