package fake

import (
	"context"
	"errors"
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

// Repository is an in-memory test double for git.Repository.
type Repository struct {
	recordedHEAD string
	Worktrees    map[int]git.Worktree
	Dirty        map[int]bool
	CloneCalls   []CloneCall
}

type CloneCall struct {
	URL string
	Dir string
}

func New() *Repository {
	return &Repository{
		Worktrees: make(map[int]git.Worktree),
		Dirty:     make(map[int]bool),
	}
}

func (r *Repository) Clone(ctx context.Context, url, dir string) error {
	r.CloneCalls = append(r.CloneCalls, CloneCall{URL: url, Dir: dir})
	return nil
}

func (r *Repository) WorktreePath() string {
	return ".agent/worktrees"
}

func (r *Repository) BranchName(iss issue.Issue) string {
	return git.BranchName(iss)
}

func (r *Repository) PrepareWorktree(ctx context.Context, iss issue.Issue) (git.Worktree, error) {
	if wt, ok := r.Worktrees[iss.Number]; ok {
		return wt, nil
	}
	wt := git.Worktree{
		Path:   fmt.Sprintf("%s/%s", r.WorktreePath(), git.WorktreeDirName(iss)),
		Branch: git.BranchName(iss),
	}
	r.Worktrees[iss.Number] = wt
	return wt, nil
}

func (r *Repository) RecordBaseHEAD(ctx context.Context) error {
	if r.recordedHEAD == "" {
		r.recordedHEAD = "deadbeef"
	}
	return nil
}

func (r *Repository) BaseHEAD(ctx context.Context) (string, error) {
	if r.recordedHEAD == "" {
		return "", errors.New("git/fake: base HEAD not recorded")
	}
	return r.recordedHEAD, nil
}

func (r *Repository) IsDirty(ctx context.Context, iss issue.Issue) (bool, error) {
	if _, ok := r.Worktrees[iss.Number]; !ok {
		return false, fmt.Errorf("git/fake: no worktree for issue %d", iss.Number)
	}
	return r.Dirty[iss.Number], nil
}

func (r *Repository) RemoveWorktree(ctx context.Context, iss issue.Issue) error {
	if r.Dirty[iss.Number] {
		return git.ErrWorktreeDirty
	}
	delete(r.Worktrees, iss.Number)
	return nil
}
