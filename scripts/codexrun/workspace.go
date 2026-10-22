package codexrun

import (
	"context"
	"os"
	"path/filepath"
)

func (r Runner) writerRecord(ctx context.Context, record Record, records []Record) (Record, error) {
	repo := r.repository(r.Root)

	err := repo.ValidateBranch(ctx, record.Branch)
	if err != nil {
		return record, refuse(err.Error())
	}

	if record.Kind == kindCode {
		record, err = r.codeWorkspace(ctx, record)
	} else {
		record, err = r.fixWorkspace(ctx, record, records)
	}

	if err != nil {
		return record, err
	}

	record.Workdir = record.Worktree

	err = r.checkWriter(ctx, record)
	if err != nil {
		return record, err
	}

	return record, r.requireIdle(ctx, record)
}

func owned(records []Record, tree, branch string) bool {
	for _, record := range records {
		if record.Kind == kindCode && record.Worktree == tree && record.Branch == branch {
			return true
		}
	}

	return false
}

func (r Runner) checkWriter(ctx context.Context, record Record) error {
	if !record.writer() {
		return nil
	}

	if !r.recordedWorkspace(record) {
		return refuse("writer must stay in the recorded worktree")
	}

	actual, err := r.repository(r.Root).WorktreeOf(ctx, record.Branch)
	if err != nil {
		return refuse(err.Error())
	}

	if canRecreate(record, actual) {
		return nil
	}

	if actual != record.Worktree {
		return refuse("branch is no longer in its recorded worktree")
	}

	resolved, err := filepath.EvalSymlinks(record.Worktree)
	if err != nil || resolved != record.Worktree {
		return refuse("worktree path changed or is a symlink")
	}

	return nil
}

func (r Runner) prepareWorkspace(ctx context.Context, record Record) error {
	if record.Kind != kindCode {
		return nil
	}

	_, err := os.Stat(record.Worktree)
	if err == nil {
		return nil
	}

	repo := r.repository(r.Root)

	return repo.AddWorktree(ctx, record.Worktree, record.Branch, record.Since, repo.BranchExists(ctx, record.Branch))
}

func (r Runner) recordedWorkspace(record Record) bool {
	return filepath.Dir(record.Worktree) == filepath.Join(r.Root, "tmp", "codex-worktrees") &&
		record.Workdir == record.Worktree
}

func canRecreate(record Record, actual string) bool {
	_, err := os.Lstat(record.Worktree)

	return record.Kind == kindCode && record.SessionID == "" && os.IsNotExist(err) && actual == ""
}

func (r Runner) codeWorkspace(ctx context.Context, record Record) (Record, error) {
	repo := r.repository(r.Root)
	if repo.BranchExists(ctx, record.Branch) {
		return record, refuse("branch exists; use a fix run")
	}

	since, err := repo.ResolveCommit(ctx, record.Base)
	if err != nil {
		return record, refuse(err.Error())
	}

	record.Since = since
	record.Worktree = filepath.Join(r.Root, "tmp", "codex-worktrees", record.Name)

	_, err = os.Lstat(record.Worktree)
	if err == nil {
		return record, refuse("worktree path exists")
	}

	return record, nil
}

func (r Runner) fixWorkspace(ctx context.Context, record Record, records []Record) (Record, error) {
	repo := r.repository(r.Root)

	tree, err := repo.WorktreeOf(ctx, record.Branch)
	if err != nil {
		return record, refuse(err.Error())
	}

	if !owned(records, tree, record.Branch) {
		return record, refuse("no code run records this branch and worktree")
	}

	record.Worktree = tree

	record.Since, err = repo.ResolveCommit(ctx, "refs/heads/"+record.Branch)
	if err != nil {
		return record, refuse(err.Error())
	}

	return record, nil
}
