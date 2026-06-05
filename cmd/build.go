/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	issuepkg "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run the AFK build workflow for an issue",
	RunE:  runBuild,
}

type buildOptions struct {
	IssueID           string
	IdleTimeout       time.Duration
	CompletionTimeout time.Duration
}

type buildRuntimeOptions struct {
	buildOptions
	RepoPath                     string
	Model                        string
	Variant                      string
	AgentName                    string
	SandboxMode                  string
	DangerouslySkipPermissions   bool
}

type buildWorkflowDeps struct {
	Issues  issuepkg.IssueTracker
	Repo    git.Repository
	Sandbox sandbox.SandboxProvider
	Agent   agent.AgentProvider
	Prompt  prompt.Builder
}

var (
	buildDepsResolver  = resolveBuildWorkflowDeps
	buildWorkflowRunner = runBuildWorkflow
)

func runBuild(cmd *cobra.Command, args []string) error {
	issueID, err := resolveIssue(args)
	if err != nil {
		return err
	}

	opts := buildRuntimeOptions{
		buildOptions: buildOptions{
			IssueID:           issueID,
			IdleTimeout:       buildIdleTimeout,
			CompletionTimeout: buildCompletionTimeout,
		},
		RepoPath:                   buildRepoPath,
		Model:                      buildModel,
		Variant:                    buildVariant,
		AgentName:                  buildAgentName,
		SandboxMode:                buildSandboxMode,
		DangerouslySkipPermissions: buildDangerouslySkipPermissions,
	}

	deps, err := buildDepsResolver(opts)
	if err != nil {
		return err
	}

	summary, err := buildWorkflowRunner(cmd.Context(), deps, opts.buildOptions)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"build completed for issue #%d with %s in %s\n",
		summary.IssueNumber,
		summary.AgentName,
		summary.SandboxKind,
	)
	return nil
}

func runBuildWorkflow(ctx context.Context, deps buildWorkflowDeps, opts buildOptions) (summary orchestrator.RunSummary, err error) {
	it, err := deps.Issues.ReadIssue(ctx, opts.IssueID)
	if err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("read issue: %w", err)
	}

	if err := deps.Issues.Lock(ctx, opts.IssueID); err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("lock issue: %w", err)
	}
	defer func() {
		unlockErr := deps.Issues.Unlock(ctx, opts.IssueID)
		if unlockErr == nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("unlock issue: %w", unlockErr)
			return
		}
		err = errors.Join(err, fmt.Errorf("unlock issue: %w", unlockErr))
	}()

	if err := deps.Repo.RecordBaseHEAD(ctx); err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("record base head: %w", err)
	}
	wt, err := deps.Repo.PrepareWorktree(ctx, *it)
	if err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("prepare worktree: %w", err)
	}

	o := orchestrator.New(orchestrator.Deps{
		Agent:   deps.Agent,
		Sandbox: deps.Sandbox,
		Issues:  deps.Issues,
	})
	summary, err = o.Run(ctx, orchestrator.RunInput{
		IssueID:           opts.IssueID,
		Issue:             it,
		Prompt:            deps.Prompt.ForIssue(*it),
		Workspace:         wt.Path,
		IdleTimeout:       opts.IdleTimeout,
		CompletionTimeout: opts.CompletionTimeout,
	})

	commentBody := renderBuildComment(*it, wt, summary, err)
	if commentErr := deps.Issues.Comment(ctx, opts.IssueID, commentBody); commentErr != nil {
		if err == nil {
			err = fmt.Errorf("comment on issue: %w", commentErr)
		} else {
			err = errors.Join(err, fmt.Errorf("comment on issue: %w", commentErr))
		}
	}
	return summary, err
}

func renderBuildComment(it issuepkg.Issue, wt git.Worktree, summary orchestrator.RunSummary, runErr error) string {
	status := "succeeded"
	details := ""
	if runErr != nil {
		status = "failed"
		details = fmt.Sprintf("\nError: `%s`", runErr)
	}
	timeoutLine := ""
	if summary.TimeoutKind != "" {
		timeoutLine = fmt.Sprintf("\nTimeout: `%s`", summary.TimeoutKind)
	}
	return fmt.Sprintf(
		"Build %s for issue #%d.\n\nCompleted: `%t`\nSuccess: `%t`\nBranch: `%s`\nWorktree: `%s`\nAgent: `%s`\nSandbox: `%s`%s%s",
		status,
		it.Number,
		summary.Completed,
		summary.Success,
		wt.Branch,
		wt.Path,
		summary.AgentName,
		summary.SandboxKind,
		timeoutLine,
		details,
	)
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
