package inventory_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
)

// scanFixture materializes a harness fixture and scans it.
func scanFixture(t *testing.T, fixture string) inventory.Snapshot {
	t.Helper()

	return inventory.Scan(materialize(t, fixture))
}

// wantDoc asserts a chip's status.
func wantDoc(t *testing.T, snap inventory.Snapshot, key, status string) {
	t.Helper()

	got, ok := snap.Documents[key]
	if !ok {
		t.Fatalf("document chip %q not found in snapshot", key)
	}

	if got != status {
		t.Fatalf("chip %q status = %q, want %q (error=%q)", key, got, status, snap.DocumentErrors[key])
	}
}

// findEpic returns the epic row for a filename, failing if absent.
func findEpic(t *testing.T, snap inventory.Snapshot, file string) inventory.Epic {
	t.Helper()

	for _, epic := range snap.Epics {
		if epic.File == file {
			return epic
		}
	}

	t.Fatalf("epic row %q not found in snapshot", file)

	return inventory.Epic{}
}

// findStory returns the story row for a position-derived create id.
func findStory(t *testing.T, snap inventory.Snapshot, createID string) inventory.Story {
	t.Helper()

	for _, epic := range snap.Epics {
		for _, story := range epic.Stories {
			if story.CreateID == createID {
				return story
			}
		}
	}

	t.Fatalf("story row %q not found in snapshot", createID)

	return inventory.Story{}
}

func TestScanDocumentChips(t *testing.T) {
	t.Parallel()

	cases := []struct {
		fixture string
		key     string
		status  string
	}{
		{"p4-config-invalid", "config", inventory.StatusInvalid},
		{"p4-stories-not-a-dir", "stories-dir", inventory.StatusNotADir},
		{"p4-checklist-missing", "checklist-us-apply", inventory.StatusMissing},
		{"p4-checklist-invalid", "checklist-us-refine", inventory.StatusInvalid},
		{"p4-registry-present-empty", "registry", inventory.StatusPresentEmpty},
	}

	for _, testCase := range cases {
		t.Run(testCase.fixture, func(t *testing.T) {
			t.Parallel()

			snap := scanFixture(t, testCase.fixture)
			wantDoc(t, snap, testCase.key, testCase.status)
		})
	}
}

func TestScanChecklistSiblingsStayPresent(t *testing.T) {
	t.Parallel()

	missing := scanFixture(t, "p4-checklist-missing")
	wantDoc(t, missing, "checklist-us-apply", inventory.StatusMissing)
	wantDoc(t, missing, "checklist-us-create", inventory.StatusPresent)

	invalid := scanFixture(t, "p4-checklist-invalid")
	wantDoc(t, invalid, "checklist-us-refine", inventory.StatusInvalid)
	wantDoc(t, invalid, "checklist-us-create", inventory.StatusPresent)
}

func TestScanArchitecturePathMismatch(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-architecture-path-mismatch")

	if !snap.ArchitecturePathMismatch {
		t.Fatal("expected architecture_path_mismatch = true")
	}

	if snap.ConfiguredArchitecturePath != "docs/arch/architecture.yaml" {
		t.Fatalf("configured architecture path = %q", snap.ConfiguredArchitecturePath)
	}

	wantDoc(t, snap, "architecture", inventory.StatusPresent)
}

func TestScanEpicMalformed(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-epic-malformed")
	epic := findEpic(t, snap, "epic-70-broken.yaml")

	if epic.Number != 70 {
		t.Fatalf("epic number = %d, want 70", epic.Number)
	}

	if epic.Status != inventory.EpicInvalid {
		t.Fatalf("epic status = %q, want invalid", epic.Status)
	}
}

func TestScanEpicDuplicateNumbers(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-epic-duplicate-numbers")

	for _, file := range []string{"epic-07-alpha.yaml", "epic-07-beta.yaml"} {
		epic := findEpic(t, snap, file)
		if epic.Number != 7 {
			t.Fatalf("%s number = %d, want 7", file, epic.Number)
		}

		if !epic.DuplicateNumber {
			t.Fatalf("%s expected duplicate_number flag", file)
		}
	}
}

func TestScanEpicIDMismatchExposesIdentityTuple(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-epic-id-mismatch")
	epic := findEpic(t, snap, "epic-42-identity-mismatch.yaml")

	if !epic.IDMismatch {
		t.Fatalf("expected epic id_mismatch (doc_id=%d, number=%d)", epic.DocID, epic.Number)
	}

	story := findStory(t, snap, "42.1")
	if story.DeclaredID != "77.5" {
		t.Fatalf("declared id = %q, want 77.5", story.DeclaredID)
	}

	if story.FileID != "88.9" {
		t.Fatalf("file id = %q, want 88.9", story.FileID)
	}

	if story.Created != inventory.CreatedOne {
		t.Fatalf("created = %q, want one", story.Created)
	}

	if !story.Flags.IDMismatch {
		t.Fatal("expected story id_mismatch flag (88.9 != 77.5)")
	}
}

func TestScanStoryDuplicateDeclaredIDs(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-story-duplicate-ids")

	for _, createID := range []string{"70.1", "70.2"} {
		story := findStory(t, snap, createID)
		if !story.Flags.DuplicateDeclaredID {
			t.Fatalf("story %q expected duplicate_declared_id flag", createID)
		}
	}
}

func TestScanStoryCreatedStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		fixture string
		created string
		reason  string
	}{
		{"p4-story-ambiguous-files", inventory.CreatedAmbiguous, inventory.ReasonAmbiguous},
		{"p4-story-malformed", inventory.CreatedInvalid, inventory.ReasonInvalid},
	}

	for _, testCase := range cases {
		t.Run(testCase.fixture, func(t *testing.T) {
			t.Parallel()

			story := findStory(t, scanFixture(t, testCase.fixture), "70.1")
			if story.Created != testCase.created {
				t.Fatalf("created = %q, want %q", story.Created, testCase.created)
			}

			if story.Applied.Status != inventory.AppliedUnknown {
				t.Fatalf("applied status = %q, want unknown", story.Applied.Status)
			}

			if story.Applied.Reason != testCase.reason {
				t.Fatalf("applied reason = %q, want %q", story.Applied.Reason, testCase.reason)
			}
		})
	}
}

func TestScanStoryApplyIneligibility(t *testing.T) {
	t.Parallel()

	cases := []struct {
		fixture string
		reason  string
		flag    func(inventory.StoryFlags) bool
	}{
		{
			"p4-story-deprecated-format", inventory.ReasonDeprecatedFormat,
			func(f inventory.StoryFlags) bool { return f.DeprecatedFormat },
		},
		{
			"p4-story-no-acs", inventory.ReasonNoAcceptanceCriteria,
			func(f inventory.StoryFlags) bool { return f.NoACs },
		},
		{
			"p4-story-empty-internal-id", inventory.ReasonEmptyInternalID,
			func(f inventory.StoryFlags) bool { return f.EmptyInternalID },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.fixture, func(t *testing.T) {
			t.Parallel()

			story := findStory(t, scanFixture(t, testCase.fixture), "70.1")
			if story.Created != inventory.CreatedOne {
				t.Fatalf("created = %q, want one", story.Created)
			}

			if !testCase.flag(story.Flags) {
				t.Fatalf("expected eligibility flag set for %s", testCase.fixture)
			}

			if story.Applied.Status != inventory.AppliedUnknown {
				t.Fatalf("applied status = %q, want unknown", story.Applied.Status)
			}

			if story.Applied.Reason != testCase.reason {
				t.Fatalf("applied reason = %q, want %q", story.Applied.Reason, testCase.reason)
			}
		})
	}
}
