package git

import (
	"context"
	"errors"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

// ErrWorktreeDirty is returned when an operation would destroy uncommitted work.
var ErrWorktreeDirty = errors.New("git: worktree has uncommitted changes")

// Worktree describes a prepared issue-scoped working tree.
type Worktree struct {
	Path   string
	Branch string
}

// Repository abstracts local git operations used by workflows.
type Repository interface {
	Clone(ctx context.Context, url, dir string) error
	WorktreePath() string

	BranchName(iss issue.Issue) string
	PrepareWorktree(ctx context.Context, iss issue.Issue) (Worktree, error)
	RecordBaseHEAD(ctx context.Context) error
	BaseHEAD(ctx context.Context) (string, error)
	IsDirty(ctx context.Context, iss issue.Issue) (bool, error)
	RemoveWorktree(ctx context.Context, iss issue.Issue) error
}
