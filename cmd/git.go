package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git/local"
	issuetrack "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/spf13/cobra"
)

var (
	gitRepoPath   string
	gitIssueNum   int
	gitIssueTitle string
)

var gitCmd = &cobra.Command{
	Use:    "git",
	Short:  "Inspect git worktree lifecycle (maintainer debug)",
	Hidden: true,
}

var gitPrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Create or reuse an issue worktree",
	RunE:  runGitPrepare,
}

var gitRecordBaseHEADCmd = &cobra.Command{
	Use:   "record-base-head",
	Short: "Record default-branch HEAD as the pre-run baseline",
	RunE:  runGitRecordBaseHEAD,
}

var gitBaseHEADCmd = &cobra.Command{
	Use:   "base-head",
	Short: "Print the recorded base HEAD",
	RunE:  runGitBaseHEAD,
}

var gitDirtyCmd = &cobra.Command{
	Use:   "dirty",
	Short: "Report whether the issue worktree has uncommitted changes",
	RunE:  runGitDirty,
}

var gitRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the issue worktree when it is clean",
	RunE:  runGitRemove,
}

func init() {
	rootCmd.AddCommand(gitCmd)

	gitCmd.PersistentFlags().StringVar(&gitRepoPath, "repo", "", "Path to the target git repository root")
	_ = gitCmd.MarkPersistentFlagRequired("repo")

	issueFlags := func(c *cobra.Command) {
		c.Flags().IntVarP(&gitIssueNum, "number", "n", 0, "Issue number")
		c.Flags().StringVar(&gitIssueTitle, "title", "", "Issue title (for branch slug)")
		_ = c.MarkFlagRequired("number")
		_ = c.MarkFlagRequired("title")
	}

	issueFlags(gitPrepareCmd)
	issueFlags(gitDirtyCmd)
	issueFlags(gitRemoveCmd)

	gitCmd.AddCommand(gitPrepareCmd, gitRecordBaseHEADCmd, gitBaseHEADCmd, gitDirtyCmd, gitRemoveCmd)
}

func runGitPrepare(cmd *cobra.Command, args []string) error {
	repo, iss, err := gitLocalRepoAndIssue()
	if err != nil {
		return err
	}
	wt, err := repo.PrepareWorktree(context.Background(), iss)
	if err != nil {
		return err
	}
	fmt.Printf("branch: %s\npath: %s\nworktree_root: %s\n", wt.Branch, wt.Path, repo.WorktreePath())
	return nil
}

func runGitRecordBaseHEAD(cmd *cobra.Command, args []string) error {
	repo, err := gitLocalRepo()
	if err != nil {
		return err
	}
	if err := repo.RecordBaseHEAD(context.Background()); err != nil {
		return err
	}
	sha, err := repo.BaseHEAD(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(sha)
	return nil
}

func runGitBaseHEAD(cmd *cobra.Command, args []string) error {
	repo, err := gitLocalRepo()
	if err != nil {
		return err
	}
	sha, err := repo.BaseHEAD(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(sha)
	return nil
}

func runGitDirty(cmd *cobra.Command, args []string) error {
	repo, iss, err := gitLocalRepoAndIssue()
	if err != nil {
		return err
	}
	dirty, err := repo.IsDirty(context.Background(), iss)
	if err != nil {
		return err
	}
	fmt.Println(dirty)
	return nil
}

func runGitRemove(cmd *cobra.Command, args []string) error {
	repo, iss, err := gitLocalRepoAndIssue()
	if err != nil {
		return err
	}
	err = repo.RemoveWorktree(context.Background(), iss)
	if errors.Is(err, git.ErrWorktreeDirty) {
		fmt.Fprintln(os.Stderr, "worktree is dirty; not removed")
		return err
	}
	if err != nil {
		return err
	}
	fmt.Println("removed")
	return nil
}

func gitLocalRepo() (*local.Repository, error) {
	if gitRepoPath == "" {
		return nil, fmt.Errorf("--repo is required")
	}
	return local.New(gitRepoPath), nil
}

func gitLocalRepoAndIssue() (*local.Repository, issuetrack.Issue, error) {
	repo, err := gitLocalRepo()
	if err != nil {
		return nil, issuetrack.Issue{}, err
	}
	iss, err := gitIssueFromFlags()
	if err != nil {
		return nil, issuetrack.Issue{}, err
	}
	return repo, iss, nil
}

func gitIssueFromFlags() (issuetrack.Issue, error) {
	if gitIssueNum <= 0 {
		return issuetrack.Issue{}, fmt.Errorf("--number must be a positive issue number")
	}
	if gitIssueTitle == "" {
		return issuetrack.Issue{}, fmt.Errorf("--title is required")
	}
	return issuetrack.Issue{Number: gitIssueNum, Title: gitIssueTitle}, nil
}
