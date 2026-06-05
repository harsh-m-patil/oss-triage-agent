/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	issuepkg "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/logging"
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
	IssueID     string
	IdleTimeout time.Duration
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
	Log     *logging.CharmLogger
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
			IssueID:     issueID,
			IdleTimeout: buildIdleTimeout,
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
	deps.Log = logging.NewCharm(cmd.ErrOrStderr(), "build")

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
	log := deps.Log.Charm()
	log.Info("reading issue", "issue_id", opts.IssueID)
	it, err := deps.Issues.ReadIssue(ctx, opts.IssueID)
	if err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("read issue: %w", err)
	}
	log.Info("loaded issue", "number", it.Number, "title", singleLine(it.Title))

	log.Info("locking issue", "number", it.Number)
	if err := deps.Issues.Lock(ctx, opts.IssueID); err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("lock issue: %w", err)
	}
	defer func() {
		log.Info("unlocking issue", "number", it.Number)
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

	log.Info("recording base HEAD")
	if err := deps.Repo.RecordBaseHEAD(ctx); err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("record base head: %w", err)
	}
	log.Info("preparing worktree", "branch", deps.Repo.BranchName(*it))
	wt, err := deps.Repo.PrepareWorktree(ctx, *it)
	if err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("prepare worktree: %w", err)
	}
	log.Info("worktree ready", "path", wt.Path)

	o := orchestrator.New(orchestrator.Deps{
		Agent:   deps.Agent,
		Sandbox: deps.Sandbox,
		Issues:  deps.Issues,
	})
	log.Info(
		"starting agent",
		"agent", deps.Agent.Name(),
		"workspace", wt.Path,
		"idle_timeout", opts.IdleTimeout,
	)
	summary, err = o.Run(ctx, orchestrator.RunInput{
		IssueID:     opts.IssueID,
		Issue:       it,
		Prompt:      deps.Prompt.ForIssue(*it),
		Workspace:   wt.Path,
		IdleTimeout: opts.IdleTimeout,
		Progress:    newBuildProgressLogger(deps.Log),
	})
	log.Info(
		"agent finished",
		"completed", summary.Completed,
		"success", summary.Success,
		"sandbox", summary.SandboxKind,
		"events", len(summary.Events),
	)

	commentBody := renderBuildComment(*it, wt, summary, err)
	log.Info("posting issue comment", "number", it.Number)
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

func newBuildProgressLogger(log *logging.CharmLogger) func(orchestrator.ProgressEvent) {
	if log == nil {
		return nil
	}
	return func(ev orchestrator.ProgressEvent) {
		switch ev.Kind {
		case orchestrator.ProgressAgentStart:
			log.Agent("command", "command", singleLine(joinCommand(ev.Command, ev.Args)))
		case orchestrator.ProgressAgentEvent:
			if ev.Event == nil {
				return
			}
			switch ev.Event.Kind {
			case agent.EventSessionID:
				log.Agent("session", "session_id", ev.Event.SessionID)
			case agent.EventText:
				log.Agent("text", "text", truncateForLog(singleLine(ev.Event.Text), 200))
			case agent.EventResult:
				if ev.Event.Result == nil {
					return
				}
				log.Agent("result", "output", truncateForLog(singleLine(ev.Event.Result.Output), 200))
			case agent.EventToolCall:
				if ev.Event.ToolCall == nil {
					return
				}
				log.Tool(
					ev.Event.ToolCall.Name,
					"args", truncateForLog(singleLine(ev.Event.ToolCall.Args), 160),
				)
			case agent.EventUsage:
				if ev.Event.Usage == nil {
					return
				}
				log.Usage(
					"tokens",
					"input_tokens", ev.Event.Usage.InputTokens,
					"output_tokens", ev.Event.Usage.OutputTokens,
				)
			}
		case orchestrator.ProgressAgentStderr:
			log.Stderr("line", "line", truncateForLog(singleLine(ev.StderrLine), 200))
		case orchestrator.ProgressHeartbeat:
			wait := ev.Wait.Round(time.Second)
			if wait <= 0 {
				wait = time.Second
			}
			if ev.Completed {
				log.Heartbeat("waiting for shutdown", "since_last_stdout", wait)
				return
			}
			log.Heartbeat("waiting for output", "since_last_stdout", wait)
		}
	}
}

func joinCommand(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, command)
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
