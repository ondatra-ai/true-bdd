package release

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// BuildTag builds a release tag's artifacts, stamped with the tag's version, from HEAD,
// refusing a tag of another form or one that names another commit.
func BuildTag(ctx context.Context, root, tag, out string) error {
	version, err := TagVersion(tag)
	if err != nil {
		return err
	}

	repository := git.Repository{Dir: root}

	tagged, err := repository.ResolveCommit(ctx, tag)
	if err != nil {
		return fmt.Errorf("resolving release tag: %w", err)
	}

	head, err := repository.ResolveCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving build commit: %w", err)
	}

	if tagged != head {
		return fmt.Errorf("%w: tag %s names %s, not HEAD %s", errRefused, tag, tagged, head)
	}

	return Build(ctx, root, version, out)
}

// Build publishes four archives and checksums from HEAD, never the worktree's files,
// each binary stamped with the version and that commit (#220).
func Build(ctx context.Context, root, version, out string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`).MatchString(version) {
		return fmt.Errorf("%w: --version must be an artifact-safe name", errRefused)
	}

	repository := git.Repository{Dir: root}

	commit, err := repository.ResolveCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving build commit: %w", err)
	}

	err = disk.Dir(filepath.Join(root, "tmp"), disk.Private)
	if err != nil {
		return fmt.Errorf("preparing scratch: %w", err)
	}

	scratch, err := disk.TempDir(filepath.Join(root, "tmp"), "release-build-*")
	if err != nil {
		return fmt.Errorf("preparing build export: %w", err)
	}

	defer func() { _ = disk.RemoveTree(scratch) }()

	source := filepath.Join(scratch, "source")

	err = exportSource(ctx, root, commit, source)
	if err != nil {
		return err
	}

	err = disk.Dir(out, disk.Private)
	if err != nil {
		return fmt.Errorf("creating artifacts: %w", err)
	}

	checksums, err := buildPlatforms(ctx, source, scratch, gotool.Stamp{Version: version, Commit: commit}, out)
	if err != nil {
		return err
	}

	err = writeBuildMetadata(out, Metadata{Version: version, Commit: commit}, checksums)
	if err != nil {
		return err
	}

	slog.Info("Release artifacts built", "commit", commit, "version", version, "artifacts", out)

	return nil
}

func buildPlatforms(ctx context.Context, source, scratch string, stamp gotool.Stamp, out string) (string, error) {
	var checksums strings.Builder

	for _, name := range archiveNames(stamp.Version) {
		parts := strings.Split(strings.TrimSuffix(name, ".tar.gz"), "_")

		const osFromEnd = 2

		targetOSIndex, targetArchIndex := len(parts)-osFromEnd, len(parts)-1

		binary := filepath.Join(scratch, "bin", "true-bdd")

		err := disk.Dir(filepath.Dir(binary), disk.Private)
		if err != nil {
			return "", fmt.Errorf("preparing binary: %w", err)
		}

		err = gotool.BuildRelease(ctx, source, binary, parts[targetOSIndex], parts[targetArchIndex], stamp)
		if err != nil {
			return "", fmt.Errorf("building archive: %w", err)
		}

		data, err := disk.Read(binary)
		if err != nil {
			return "", fmt.Errorf("reading built binary: %w", err)
		}

		archive, err := packBinary(data)
		if err != nil {
			return "", err
		}

		err = disk.Write(filepath.Join(out, name), archive, disk.Private)
		if err != nil {
			return "", fmt.Errorf("writing archive: %w", err)
		}

		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(archive), name)
		slog.Info("Release archive built", "archive", name)
	}

	return checksums.String(), nil
}

func writeBuildMetadata(out string, metadata Metadata, checksums string) error {
	err := disk.Write(filepath.Join(out, "checksums.txt"), []byte(checksums), disk.Private)
	if err != nil {
		return fmt.Errorf("writing checksums: %w", err)
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encoding release metadata: %w", err)
	}

	err = disk.Write(filepath.Join(out, "release.json"), append(data, '\n'), disk.Private)
	if err != nil {
		return fmt.Errorf("writing release metadata: %w", err)
	}

	return nil
}
