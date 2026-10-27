package release_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

// TestReadmeQuotesTheRehearsedGuide holds README.md's install guide to the commands the
// rehearsal runs, one for one and in order, so the two cannot drift apart.
func TestReadmeQuotesTheRehearsedGuide(t *testing.T) {
	t.Parallel()

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	notes, err := release.Notes(string(readme), "vX.Y.Z", "<commit>")
	if err != nil {
		t.Fatal(err)
	}

	var quoted []string

	inBlock := false

	for line := range strings.SplitSeq(notes, "\n") {
		switch {
		case strings.HasPrefix(line, "```"):
			inBlock = !inBlock
		case inBlock && strings.TrimSpace(line) != "":
			quoted = append(quoted, line)
		}
	}

	if !slices.Equal(quoted, release.GuideCommands()) {
		t.Fatalf("README's install guide quotes\n%s\nbut the rehearsal runs\n%s",
			strings.Join(quoted, "\n"), strings.Join(release.GuideCommands(), "\n"))
	}
}

func TestNotesFillTheGuideWithTheRelease(t *testing.T) {
	t.Parallel()

	readme := "# Title\n\nbefore\n<!-- install-guide:begin -->\n## Install\n\n" +
		"go install example@vX.Y.Z\ngo get lib@<commit>\n<!-- install-guide:end -->\nafter\n"

	notes, err := release.Notes(readme, "v0.1.0", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	want := "Release `services/true-bdd/v0.1.0`, commit `abc123`.\n\n" +
		"## Install\n\ngo install example@v0.1.0\ngo get lib@abc123\n"
	if notes != want {
		t.Fatalf("notes = %q, want %q", notes, want)
	}

	_, err = release.Notes("# no guide\n", "v0.1.0", "abc123")
	if err == nil {
		t.Fatal("a README without the guide markers was accepted")
	}
}
