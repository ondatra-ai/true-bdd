package stepcoverage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSteps lays down a steps package holding the given files, keyed by
// name, and returns its directory.
func writeSteps(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "steps")

	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	for name, body := range files {
		writeErr := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600)
		if writeErr != nil {
			t.Fatalf("write %s: %v", name, writeErr)
		}
	}

	return dir
}

// anyStep is the pattern most cases fold to, from a file named srcFile.
const (
	anyStep = "^a$"
	srcFile = "s.go"
)

// pkg wraps a Register body in a compilable-looking package.
func pkg(body string) string {
	return "package steps\n\nfunc Register(suite Suite) {\n" + body + "\n}\n"
}

// Every constant-expression form a suite can register must fold to the
// exact string the suite would have compiled.
func TestLoadDefinitionsFoldsConstantExpressions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"backtick literal", map[string]string{srcFile: pkg("suite.Step(`^a$`, nil)")}, anyStep},
		{"quoted literal", map[string]string{srcFile: pkg(`suite.Step("^a$", nil)`)}, anyStep},
		{"concatenated literals", map[string]string{srcFile: pkg("suite.Step(`^a`+`$`, nil)")}, anyStep},
		{"parenthesised", map[string]string{srcFile: pkg("suite.Step((`^a`+`$`), nil)")}, anyStep},
		{
			"spliced const",
			map[string]string{srcFile: "package steps\n\nconst sel = `x`\n\n" +
				"func Register(suite Suite) {\nsuite.Step(`^`+sel+`$`, nil)\n}\n"},
			"^x$",
		},
		{
			"const of const, two deep",
			map[string]string{srcFile: "package steps\n\nconst inner = `x`\n\nconst outer = inner + `y`\n\n" +
				"func Register(suite Suite) {\nsuite.Step(`^`+outer+`$`, nil)\n}\n"},
			"^xy$",
		},
		{
			"const declared in a sibling file",
			map[string]string{
				"a.go": "package steps\n\nconst sel = `x`\n",
				"b.go": pkg("suite.Step(`^`+sel+`$`, nil)"),
			},
			"^x$",
		},
		{"receiver not named suite", map[string]string{
			srcFile: "package steps\n\nfunc Register(s Suite) {\ns.Step(`^a$`, nil)\n}\n"}, anyStep},
		{"call in a helper Register never reaches", map[string]string{
			srcFile: "package steps\n\nfunc orphan(suite Suite) {\nsuite.Step(`^a$`, nil)\n}\n"}, anyStep},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			defs, err := loadDefinitions(writeSteps(t, testCase.files))
			if err != nil {
				t.Fatalf("loadDefinitions: %v", err)
			}

			if len(defs) != 1 {
				t.Fatalf("got %d definitions, want 1", len(defs))
			}

			if defs[0].source != testCase.want {
				t.Errorf("source = %q, want %q", defs[0].source, testCase.want)
			}
		})
	}
}

// A pattern that does not fold is refused by name — never skipped, which
// is what would report a bound step as unbound.
func TestLoadDefinitionsRefusesNonConstantPatterns(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"call":              pkg("suite.Step(fmt.Sprintf(`^%s$`, k), nil)"),
		"cross-package ref": pkg("suite.Step(other.Pattern, nil)"),
		"package var": "package steps\n\nvar p = `^a$`\n\n" +
			"func Register(suite Suite) {\nsuite.Step(p, nil)\n}\n",
		"unknown ident":  pkg("suite.Step(missing, nil)"),
		"non-string lit": pkg("suite.Step(42, nil)"),
		"other operator": "package steps\n\nconst a = `x`\n\nconst b = `y`\n\n" +
			"func Register(suite Suite) {\nsuite.Step(a[0:1], nil)\n}\n",
		"iota const": "package steps\n\nconst (\n\ta = iota\n\tb\n)\n\n" +
			"func Register(suite Suite) {\nsuite.Step(b, nil)\n}\n",
		"const cycle": "package steps\n\nconst a = b\n\nconst b = a\n\n" +
			"func Register(suite Suite) {\nsuite.Step(a, nil)\n}\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadDefinitions(writeSteps(t, map[string]string{srcFile: body}))
			if !errors.Is(err, ErrPatternNotConstant) {
				t.Fatalf("want ErrPatternNotConstant, got %v", err)
			}

			if !strings.Contains(err.Error(), "s.go:") {
				t.Errorf("refusal must name the file and line, got %v", err)
			}
		})
	}
}

// A pattern that will not compile fails the whole answer rather than
// quietly shrinking the table.
func TestLoadDefinitionsRefusesUncompilablePattern(t *testing.T) {
	t.Parallel()

	_, err := loadDefinitions(writeSteps(t, map[string]string{srcFile: pkg("suite.Step(`^the [ page$`, nil)")}))
	if !errors.Is(err, ErrPatternInvalid) {
		t.Fatalf("want ErrPatternInvalid, got %v", err)
	}
}

// An unparseable file is an error: the definitions it registers would
// otherwise vanish and every step they bind would read as a gap.
func TestLoadDefinitionsRefusesUnparseableFile(t *testing.T) {
	t.Parallel()

	_, err := loadDefinitions(writeSteps(t, map[string]string{srcFile: "package steps\n\nfunc ("}))
	if err == nil {
		t.Fatal("want a parse error, got none")
	}
}

// What the scanner must NOT collect: a _test.go registration (the suite
// binary links the package without it) and a .Step of another arity.
func TestLoadDefinitionsIgnoresNonRegistrations(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"test file":   {"s_test.go": pkg("suite.Step(fmt.Sprintf(`^%s$`, k), nil)")},
		"wrong arity": {srcFile: pkg("suite.Step(`^a$`)")},
		"no files":    {},
	}

	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defs, err := loadDefinitions(writeSteps(t, files))
			if err != nil {
				t.Fatalf("loadDefinitions: %v", err)
			}

			if len(defs) != 0 {
				t.Fatalf("got %d definitions, want 0", len(defs))
			}
		})
	}
}

// An absent steps/ directory is zero definitions, not an error: a suite
// that has written none has every step unbound, which is what sends its
// scenarios to the walk.
func TestLoadDefinitionsTreatsAbsentDirAsEmpty(t *testing.T) {
	t.Parallel()

	defs, err := loadDefinitions(filepath.Join(t.TempDir(), "nope", "steps"))
	if err != nil {
		t.Fatalf("loadDefinitions: %v", err)
	}

	if len(defs) != 0 {
		t.Fatalf("got %d definitions, want 0", len(defs))
	}
}
