package taskhandle_test

import (
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

const claudeMD = "CLAUDE.md"

func TestParseGlobsAcceptsBothSeparators(t *testing.T) {
	t.Parallel()

	for name, field := range map[string]string{
		"newlines": "scripts/**\ndocs/**\n",
		"commas":   "scripts/**, docs/**",
		"mixed":    "scripts/**,\n  docs/**  \n\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			want := []string{"scripts/**", "docs/**"}
			if got := taskhandle.ParseGlobs(field); !reflect.DeepEqual(got, want) {
				t.Errorf("ParseGlobs(%q) = %v, want %v", field, got, want)
			}
		})
	}
}

func TestOutOfScope(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		globs   []string
		changed []string
		want    []string
	}{
		"a prefix glob covers what is under it": {
			[]string{"scripts/merge/**"}, []string{"scripts/merge/run.go"}, nil,
		},
		"and nothing else": {
			[]string{"scripts/merge/**"}, []string{"scripts/commit/run.go"},
			[]string{"scripts/commit/run.go"},
		},
		"an extension glob covers by suffix": {
			[]string{"**/*.go"}, []string{"scripts/x/y.go"}, nil,
		},
		"./* is the repo-wide case ticket-schema.yaml blesses": {
			[]string{"./*"}, []string{"anything/at/all.md", "CLAUDE.md"}, nil,
		},
		"a bare directory means everything under it": {
			[]string{"docs"}, []string{"docs/adr/0003.md"}, nil,
		},
		"a trailing slash does too": {
			[]string{"docs/"}, []string{"docs/adr/0003.md"}, nil,
		},
		"a literal path matches only itself": {
			[]string{claudeMD}, []string{claudeMD, "CONTEXT.md"}, []string{"CONTEXT.md"},
		},
		"no globs means everything strays": {
			nil, []string{"scripts/x.go"}, []string{"scripts/x.go"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := taskhandle.OutOfScope(testCase.changed, testCase.globs)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("OutOfScope(%v, %v) = %v, want %v",
					testCase.changed, testCase.globs, got, testCase.want)
			}
		})
	}
}
