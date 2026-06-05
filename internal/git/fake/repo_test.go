package fake_test

import (
	"context"
	"errors"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git/fake"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

func TestFakeRepository_prepareRecordAndCleanup(t *testing.T) {
	t.Parallel()

	repo := fake.New()
	iss := issue.Issue{Number: 1, Title: "hello world"}

	if err := repo.RecordBaseHEAD(context.Background()); err != nil {
		t.Fatalf("RecordBaseHEAD: %v", err)
	}
	head, err := repo.BaseHEAD(context.Background())
	if err != nil || head == "" {
		t.Fatalf("BaseHEAD = %q, %v", head, err)
	}

	wt, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("PrepareWorktree: %v", err)
	}
	if wt.Branch != "agent/issue-1-hello-world" {
		t.Fatalf("Branch = %q", wt.Branch)
	}

	wt2, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		t.Fatalf("reuse PrepareWorktree: %v", err)
	}
	if wt2.Path != wt.Path {
		t.Fatalf("reused path changed")
	}

	repo.Dirty[iss.Number] = true
	err = repo.RemoveWorktree(context.Background(), iss)
	if !errors.Is(err, git.ErrWorktreeDirty) {
		t.Fatalf("RemoveWorktree dirty err = %v", err)
	}
	if _, ok := repo.Worktrees[iss.Number]; !ok {
		t.Fatal("dirty worktree removed from fake store")
	}

	repo.Dirty[iss.Number] = false
	if err := repo.RemoveWorktree(context.Background(), iss); err != nil {
		t.Fatalf("RemoveWorktree clean: %v", err)
	}
}
