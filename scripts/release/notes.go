package release

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// The README's install guide sits between these markers; the release notes repeat it.
const (
	guideBegin = "<!-- install-guide:begin -->"
	guideEnd   = "<!-- install-guide:end -->"
)

// The guide's placeholders, which the notes fill with the release's version and commit.
const (
	versionPlaceholder = "vX.Y.Z"
	commitPlaceholder  = "<commit>"
)

// WriteNotes writes a release tag's notes: the README's install guide for that version and commit.
func WriteNotes(ctx context.Context, root, tag, out string) error {
	version, err := TagVersion(tag)
	if err != nil {
		return err
	}

	commit, err := git.Repository{Dir: root}.ResolveCommit(ctx, tag)
	if err != nil {
		return fmt.Errorf("resolving release tag: %w", err)
	}

	readme, err := disk.Read(filepath.Join(root, "README.md"))
	if err != nil {
		return fmt.Errorf("reading install guide: %w", err)
	}

	notes, err := Notes(string(readme), version, commit)
	if err != nil {
		return err
	}

	err = disk.Write(out, []byte(notes), disk.Private)
	if err != nil {
		return fmt.Errorf("writing release notes: %w", err)
	}

	return nil
}

// Notes is the install guide of readme with its version and commit placeholders filled in.
func Notes(readme, version, commit string) (string, error) {
	guide, err := installGuide(readme)
	if err != nil {
		return "", err
	}

	guide = strings.ReplaceAll(guide, versionPlaceholder, version)
	guide = strings.ReplaceAll(guide, commitPlaceholder, commit)

	return "Release `" + TagPrefix + version + "`, commit `" + commit + "`.\n\n" + guide, nil
}

func installGuide(readme string) (string, error) {
	_, rest, found := strings.Cut(readme, guideBegin)
	if !found {
		return "", fmt.Errorf("%w: README has no %s", errRefused, guideBegin)
	}

	guide, _, found := strings.Cut(rest, guideEnd)
	if !found {
		return "", fmt.Errorf("%w: README has no %s", errRefused, guideEnd)
	}

	return strings.TrimSpace(guide) + "\n", nil
}
