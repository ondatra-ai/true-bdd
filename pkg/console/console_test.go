package console_test

import (
	"bytes"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

const width = 5

// Console is byte-plain by contract: 180 steps in docs/scenarios.yaml match
// against these bytes, so a prefix or a colour code is a silent parser break.
func TestOutputIsExactlyTheBytesAsked(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		emit func(*console.Console)
		want string
	}{
		"println":   {func(c *console.Console) { c.Println("hi") }, "hi\n"},
		"print":     {func(c *console.Console) { c.Print("hi") }, "hi"},
		"printf":    {func(c *console.Console) { c.Printf("%d-%s", 2, "x") }, "2-x"},
		"blank":     {func(c *console.Console) { c.BlankLine() }, "\n"},
		"separator": {func(c *console.Console) { c.Separator("-", width) }, "-----\n"},
		"header":    {func(c *console.Console) { c.Header("T", width) }, "=====\nT\n=====\n"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			testCase.emit(console.New(&buf))

			if buf.String() != testCase.want {
				t.Fatalf("got %q, want %q", buf.String(), testCase.want)
			}
		})
	}
}
