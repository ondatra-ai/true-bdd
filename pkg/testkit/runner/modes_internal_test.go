package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestParseModesReadsBothCallers(t *testing.T) {
	t.Parallel()

	modes, err := ParseModes("services:replay,tests:live")
	if err != nil {
		t.Fatalf("ParseModes: %v", err)
	}

	if modes.Services != ProxyModeReplay || modes.Tests != ProxyModeLive {
		t.Fatalf("got %+v, want services=replay tests=live", modes)
	}

	if !modes.Set() {
		t.Fatal("Set() = false for a supplied spec")
	}
}

// An absent flag is not a malformed one: the coverage guards run without
// it and bring no harness up.
func TestParseModesAcceptsAbsence(t *testing.T) {
	t.Parallel()

	modes, err := ParseModes("")
	if err != nil {
		t.Fatalf("ParseModes(\"\"): %v", err)
	}

	if modes.Set() {
		t.Fatalf("Set() = true for the absent flag: %+v", modes)
	}
}

func TestParseModesRefuses(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{
		"replay",                        // the old shorthand, now refused
		"services:replay",               // tests left unnamed
		"tests:replay",                  // services left unnamed
		"engine:replay,tests:live",      // no such caller
		"services:replay,tests:mock",    // no such mode
		"services:replay,services:live", // named twice
	} {
		_, err := ParseModes(spec)
		if !errors.Is(err, ErrModeMalformed) {
			t.Errorf("ParseModes(%q) = %v, want ErrModeMalformed", spec, err)
		}
	}
}

// Every refusal quotes the accepted spelling, so the message carries its
// own fix rather than sending a reader to the docs.
func TestParseModesRefusalNamesTheForm(t *testing.T) {
	t.Parallel()

	_, err := ParseModes("services:replay")
	if err == nil {
		t.Fatal("ParseModes(\"services:replay\") = nil error")
	}

	if want := ModeSpecForm(); !strings.Contains(err.Error(), want) {
		t.Fatalf("refusal %q does not quote %q", err, want)
	}
}

func TestModesStringRoundTrips(t *testing.T) {
	t.Parallel()

	spec := "services:record,tests:replay"

	modes, err := ParseModes(spec)
	if err != nil {
		t.Fatalf("ParseModes: %v", err)
	}

	if modes.String() != spec {
		t.Fatalf("String() = %q, want %q", modes.String(), spec)
	}
}
