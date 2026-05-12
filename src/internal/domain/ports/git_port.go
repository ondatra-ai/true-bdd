package ports

import "context"

// GitStatusChecker checks git repository status.
type GitStatusChecker interface {
	IsGitRepository(ctx context.Context) (bool, error)
	IsDetachedHead(ctx context.Context) (bool, error)
	GetCurrentBranch(ctx context.Context) (string, error)
	IsWorkingTreeClean(ctx context.Context) (bool, error)
	IsMainBehindOrigin(ctx context.Context) (bool, error)
}

// GitBranchQuery queries information about git branches.
type GitBranchQuery interface {
	LocalBranchExists(ctx context.Context, branchName string) (bool, error)
	RemoteBranchExists(ctx context.Context, branchName string) (bool, error)
}

// GitBranchOperations performs operations on git branches.
type GitBranchOperations interface {
	CheckoutRemoteBranch(ctx context.Context, branchName string) error
	SwitchBranch(ctx context.Context, branchName string) error
	CreateBranch(ctx context.Context, branchName string) error
	ForceRecreateBranch(ctx context.Context, branchName string) error
	PushBranch(ctx context.Context, branchName string) error
}

// GitPort defines the interface for git operations.
// This port interface represents the contract for git operations in the domain layer.
type GitPort interface {
	GitStatusChecker
	GitBranchQuery
	GitBranchOperations
}
