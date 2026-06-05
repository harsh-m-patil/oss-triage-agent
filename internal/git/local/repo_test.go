package local_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git/local"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

func TestRepository_PrepareWorktree_createsBranchAndPath(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo := local.New(root)
	iss := issue.Issue{Number: 3, Title: "Managed git worktrees with agent/issue-N branches"}

	wt, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("PrepareWorktree: %v", err)
	}

	wantBranch := "agent/issue-3-managed-git-worktrees-with-agent-issue-n"
	wantPath := filepath.Join(root, ".agent/worktrees", "issue-3-managed-git-worktrees-with-agent-issue-n")

	if wt.Branch != wantBranch {
		t.Fatalf("Branch = %q, want %q", wt.Branch, wantBranch)
	}
	if wt.Path != wantPath {
		t.Fatalf("Path = %q, want %q", wt.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("worktree path missing: %v", err)
	}
	if out := gitBranch(t, root, wt.Branch); out == "" {
		t.Fatalf("branch %q not found", wt.Branch)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	gitCmd(t, dir, "config", "user.name", "Test User")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "README.md")
	gitCmd(t, dir, "commit", "-m", "init")
	return dir
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", args, err, out)
	}
}

func TestRepository_RecordBaseHEAD_returnsDefaultBranchHead(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo := local.New(root)

	if err := repo.RecordBaseHEAD(context.Background()); err != nil {
		t.Fatalf("RecordBaseHEAD: %v", err)
	}
	base, err := repo.BaseHEAD(context.Background())
	if err != nil {
		t.Fatalf("BaseHEAD: %v", err)
	}

	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "README.md")
	gitCmd(t, root, "commit", "-m", "second")

	after, err := repo.BaseHEAD(context.Background())
	if err != nil {
		t.Fatalf("BaseHEAD after main moved: %v", err)
	}
	if after != base {
		t.Fatalf("BaseHEAD = %q after new main commit, want frozen %q", after, base)
	}
}

func TestRepository_PrepareWorktree_reusesExistingDirtyWorktree(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo := local.New(root)
	iss := issue.Issue{Number: 7, Title: "reuse worktree"}

	wt1, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("first PrepareWorktree: %v", err)
	}

	dirtyFile := filepath.Join(wt1.Path, "wip.txt")
	if err := os.WriteFile(dirtyFile, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wt2, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("second PrepareWorktree: %v", err)
	}
	if wt2.Path != wt1.Path {
		t.Fatalf("reused path = %q, want %q", wt2.Path, wt1.Path)
	}
	if _, err := os.Stat(dirtyFile); err != nil {
		t.Fatalf("uncommitted file removed on reuse: %v", err)
	}
}

func TestRepository_RemoveWorktree_removesCleanWorktree(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo := local.New(root)
	iss := issue.Issue{Number: 9, Title: "cleanup"}

	wt, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("PrepareWorktree: %v", err)
	}

	if err := repo.RemoveWorktree(context.Background(), iss); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists after remove: %v", err)
	}
}

func TestRepository_BaseHEAD_persistsAcrossRepositoryInstances(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo1 := local.New(root)
	if err := repo1.RecordBaseHEAD(context.Background()); err != nil {
		t.Fatalf("RecordBaseHEAD: %v", err)
	}
	base1, err := repo1.BaseHEAD(context.Background())
	if err != nil {
		t.Fatalf("BaseHEAD: %v", err)
	}

	repo2 := local.New(root)
	base2, err := repo2.BaseHEAD(context.Background())
	if err != nil {
		t.Fatalf("BaseHEAD new instance: %v", err)
	}
	if base2 != base1 {
		t.Fatalf("BaseHEAD = %q, want persisted %q", base2, base1)
	}
}

func TestRepository_RemoveWorktree_dirtyWorktreePreserved(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	repo := local.New(root)
	iss := issue.Issue{Number: 11, Title: "preserve dirty"}

	wt, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("PrepareWorktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt.Path, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = repo.RemoveWorktree(context.Background(), iss)
	if !errors.Is(err, git.ErrWorktreeDirty) {
		t.Fatalf("RemoveWorktree err = %v, want %v", err, git.ErrWorktreeDirty)
	}
	if _, statErr := os.Stat(wt.Path); statErr != nil {
		t.Fatalf("dirty worktree removed: %v", statErr)
	}
}

func gitBranch(t *testing.T, dir, name string) string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", name)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
