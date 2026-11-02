package testkit_test

import (
	"errors"
	"flag"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestFlagsKeepCallerModesAndEngineOverride(t *testing.T) {
	t.Parallel()

	flags := flag.NewFlagSet("host", flag.ContinueOnError)
	options := testkit.RegisterFlags(flags)

	err := flags.Parse([]string{"-mode=tests:live,services:replay", "-engine-bin=/host/bin/service"})
	if err != nil {
		t.Fatal(err)
	}

	modes, err := options.Modes()
	if err != nil {
		t.Fatal(err)
	}

	if modes.Services != "replay" || modes.Tests != "live" || options.EngineBin != "/host/bin/service" {
		t.Fatalf("flags = %+v, modes = %+v", options, modes)
	}
}

func TestFlagsRefuseAmbiguousModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{
		"replay", "services:replay", "services:replay,tests:unknown", "services:live,services:record,tests:replay",
	} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			_, err := (&testkit.Flags{Mode: mode}).Modes()
			if err == nil {
				t.Fatal("ambiguous mode accepted")
			}
		})
	}
}

func TestGuardsAllowNoModeButScenariosRefuseIt(t *testing.T) {
	t.Parallel()

	options := &testkit.Flags{}

	modes, err := options.Modes()
	if err != nil || modes.Set() {
		t.Fatalf("guard mode = %+v, %v", modes, err)
	}

	err = options.RequireMode()
	if !errors.Is(err, testkit.ErrModeRequired) {
		t.Fatalf("missing mode = %v, want ErrModeRequired", err)
	}
}

func TestEngineOverrideRefusesRelativePaths(t *testing.T) {
	t.Parallel()

	_, err := (&testkit.Flags{EngineBin: "bin/service"}).PrebuiltEngine()
	if !errors.Is(err, testkit.ErrEngineBinRelative) {
		t.Fatalf("relative override = %v, want ErrEngineBinRelative", err)
	}
}
