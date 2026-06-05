package git_test

import (
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

func TestBranchName_longTitle_truncatesSlug(t *testing.T) {
	t.Parallel()

	iss := issue.Issue{Number: 3, Title: "Managed git worktrees with agent/issue-N branches"}
	got := git.BranchName(iss)
	want := "agent/issue-3-managed-git-worktrees-with-agent-issue-n"
	if got != want {
		t.Fatalf("BranchName = %q, want %q", got, want)
	}
}
