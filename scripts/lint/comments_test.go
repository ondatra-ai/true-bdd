package lint_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/lint"
)

// prose builds a run of n plain comment lines with the given marker.
func prose(marker string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s argument line %d", marker, i)
	}

	return strings.Join(lines, "\n")
}

// block builds a run of n indented comment lines, which are scheme, not prose.
func block(marker string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s\trow %d", marker, i)
	}

	return strings.Join(lines, "\n")
}

func TestGoPackageDocIsExemptFromTheProseCap(t *testing.T) {
	t.Parallel()

	src := prose("//", MaxProsePlusTwo) + "\npackage lint\n"

	if found := lint.ScanComments("x.go", src, true); len(found) != 0 {
		t.Errorf("package doc flagged: %+v", found)
	}
}

func TestGoOrdinaryCommentIsNotExempt(t *testing.T) {
	t.Parallel()

	src := "package lint\n\n" + prose("//", MaxProsePlusTwo) + "\nfunc F() {}\n"

	found := lint.ScanComments("x.go", src, true)
	if len(found) != 1 {
		t.Fatalf("want 1 finding, got %+v", found)
	}

	// The run starts on line 3: `package lint`, a blank, then the comment.
	if found[0].Line != 3 {
		t.Errorf("want line 3, got %d", found[0].Line)
	}

	if !strings.Contains(found[0].Text, "lines of prose") {
		t.Errorf("want a prose finding, got %q", found[0].Text)
	}
}

// The exemption is for the manual, not for scheme: past the block cap it is a
// program with worse syntax and belongs in a README.
func TestBlockCapBindsEvenOnAnExemptHeader(t *testing.T) {
	t.Parallel()

	src := block("//", lint.MaxBlock+1) + "\npackage lint\n"

	found := lint.ScanComments("x.go", src, true)
	if len(found) != 1 || !strings.Contains(found[0].Text, "block scheme") {
		t.Fatalf("want one block finding, got %+v", found)
	}
}

func TestBlockLinesAreNotProse(t *testing.T) {
	t.Parallel()

	for name, marker := range map[string]string{"go": "//", "hash": "#"} {
		src := "code()\n" + block(marker, lint.MaxBlock) + "\nmore()\n"

		if found := lint.ScanComments("x", src, marker == "//"); len(found) != 0 {
			t.Errorf("%s: %d block lines flagged: %+v", name, lint.MaxBlock, found)
		}
	}
}

// Four spaces is the other preformatted form gofmt produces.
func TestFourSpaceIndentCountsAsBlock(t *testing.T) {
	t.Parallel()

	src := "code()\n" + strings.Repeat("//    row\n", MaxProsePlusTwo) + "more()\n"

	if found := lint.ScanComments("x.go", src, true); len(found) != 0 {
		t.Errorf("four-space indent counted as prose: %+v", found)
	}
}

func TestShellHeaderIsExemptAndAShebangDoesNotEndIt(t *testing.T) {
	t.Parallel()

	src := "#!/usr/bin/env bash\n" + prose("#", MaxProsePlusTwo) + "\nset -e\n"

	if found := lint.ScanComments("x.sh", src, false); len(found) != 0 {
		t.Errorf("header after a shebang flagged: %+v", found)
	}
}

// ci.yml opens with `name: CI`, so its first comment is not a header.
func TestCommentAfterContentIsNotAHeader(t *testing.T) {
	t.Parallel()

	src := "name: CI\n\n" + prose("#", MaxProsePlusTwo) + "\njobs:\n"

	if found := lint.ScanComments("ci.yml", src, false); len(found) != 1 {
		t.Fatalf("want 1 finding, got %+v", found)
	}
}

// A blank line ends the run but not the header: two short runs above the
// first content line are two runs, each judged on its own.
func TestBlankLineEndsTheRunButNotTheHeader(t *testing.T) {
	t.Parallel()

	src := "# one\n\n" + prose("#", MaxProsePlusTwo) + "\nkey: value\n"

	if found := lint.ScanComments("x.yaml", src, false); len(found) != 0 {
		t.Errorf("second header run flagged: %+v", found)
	}
}

func TestExactlyAtTheCapPasses(t *testing.T) {
	t.Parallel()

	src := "code()\n" + prose("//", lint.MaxProse) + "\nmore()\n"

	if found := lint.ScanComments("x.go", src, true); len(found) != 0 {
		t.Errorf("%d prose lines flagged: %+v", lint.MaxProse, found)
	}
}

func TestOneOverTheCapFails(t *testing.T) {
	t.Parallel()

	src := "code()\n" + prose("//", lint.MaxProse+1) + "\nmore()\n"

	found := lint.ScanComments("x.go", src, true)
	if len(found) != 1 {
		t.Fatalf("want 1 finding, got %+v", found)
	}

	want := fmt.Sprintf("%d lines of prose (max %d)", lint.MaxProse+1, lint.MaxProse)
	if found[0].Text != want {
		t.Errorf("want %q, got %q", want, found[0].Text)
	}
}

// A run that ends at EOF is judged too, and in Go is never a package doc.
func TestRunAtEndOfFileIsFlushed(t *testing.T) {
	t.Parallel()

	src := "code()\n" + prose("//", MaxProsePlusTwo)

	if found := lint.ScanComments("x.go", src, true); len(found) != 1 {
		t.Fatalf("trailing run not judged: %+v", found)
	}
}

func TestVendoredSkillsReadsBothProseRuns(t *testing.T) {
	t.Parallel()

	manifest := "Preamble text.\n\nEngineering: ask-matt, code-review,\ntdd, wizard.\n\n" +
		"Some other paragraph.\n\nProductivity: grill-me,\nwait-what.\n\nTrailer.\n"

	got := strings.Join(lint.VendoredSkills(manifest), ",")

	want := "ask-matt,code-review,tdd,wizard,grill-me,wait-what"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// MaxProsePlusTwo is comfortably over the cap without naming a bare number.
const MaxProsePlusTwo = lint.MaxProse + 2
