package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

// Repository runs git commands against a repo root on disk.
type Repository struct {
	Root     string
	baseHEAD string
}

func New(root string) *Repository {
	return &Repository{Root: root}
}

func (r *Repository) Clone(ctx context.Context, url, dir string) error {
	return errors.New("git/local: Clone not implemented")
}

func (r *Repository) WorktreePath() string {
	return ".agent/worktrees"
}

func (r *Repository) BranchName(iss issue.Issue) string {
	return git.BranchName(iss)
}

func (r *Repository) PrepareWorktree(ctx context.Context, iss issue.Issue) (git.Worktree, error) {
	branch := git.BranchName(iss)
	wtPath := r.worktreeAbsPath(iss)

	if _, err := os.Stat(wtPath); err == nil {
		if err := r.run(ctx, r.Root, "worktree", "repair", wtPath); err != nil {
			return git.Worktree{}, err
		}
		return git.Worktree{Path: wtPath, Branch: branch}, nil
	}

	defaultBranch, err := r.defaultBranch(ctx)
	if err != nil {
		return git.Worktree{}, err
	}

	if err := r.run(ctx, r.Root, "branch", branch, defaultBranch); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return git.Worktree{}, err
		}
	}

	if err := os.MkdirAll(filepath.Join(r.Root, r.WorktreePath()), 0o755); err != nil {
		return git.Worktree{}, err
	}

	if err := r.run(ctx, r.Root, "worktree", "add", wtPath, branch); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return git.Worktree{Path: wtPath, Branch: branch}, nil
		}
		return git.Worktree{}, err
	}

	return git.Worktree{Path: wtPath, Branch: branch}, nil
}

func (r *Repository) RecordBaseHEAD(ctx context.Context) error {
	defaultBranch, err := r.defaultBranch(ctx)
	if err != nil {
		return err
	}
	sha, err := r.revParse(ctx, defaultBranch)
	if err != nil {
		return err
	}
	r.baseHEAD = sha
	if err := os.MkdirAll(filepath.Join(r.Root, ".agent"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.baseHEADPath(), []byte(sha), 0o644)
}

func (r *Repository) BaseHEAD(ctx context.Context) (string, error) {
	if r.baseHEAD != "" {
		return r.baseHEAD, nil
	}
	data, err := os.ReadFile(r.baseHEADPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("git/local: base HEAD not recorded")
		}
		return "", err
	}
	sha := strings.TrimSpace(string(data))
	if sha == "" {
		return "", errors.New("git/local: base HEAD not recorded")
	}
	r.baseHEAD = sha
	return sha, nil
}

func (r *Repository) baseHEADPath() string {
	return filepath.Join(r.Root, ".agent", "base-head")
}

func (r *Repository) IsDirty(ctx context.Context, iss issue.Issue) (bool, error) {
	wtPath := r.worktreeAbsPath(iss)
	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("git/local: worktree %q does not exist", wtPath)
	}
	out, err := r.output(ctx, wtPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

func (r *Repository) RemoveWorktree(ctx context.Context, iss issue.Issue) error {
	dirty, err := r.IsDirty(ctx, iss)
	if err != nil {
		return err
	}
	if dirty {
		return git.ErrWorktreeDirty
	}

	wtPath := r.worktreeAbsPath(iss)
	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err := r.run(ctx, r.Root, "worktree", "remove", wtPath); err != nil {
		return err
	}
	return nil
}

func (r *Repository) worktreeAbsPath(iss issue.Issue) string {
	return filepath.Join(r.Root, r.WorktreePath(), git.WorktreeDirName(iss))
}

func (r *Repository) defaultBranch(ctx context.Context) (string, error) {
	for _, candidate := range []string{"main", "master"} {
		if _, err := r.revParse(ctx, candidate); err == nil {
			return candidate, nil
		}
	}
	out, err := r.output(ctx, r.Root, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repository) revParse(ctx context.Context, ref string) (string, error) {
	out, err := r.output(ctx, r.Root, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repository) run(ctx context.Context, dir string, args ...string) error {
	_, err := r.output(ctx, dir, args...)
	return err
}

func (r *Repository) output(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s (in %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
