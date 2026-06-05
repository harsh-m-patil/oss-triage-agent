/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	issuepkg "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/logging"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	triagepkg "github.com/harsh-m-patil/oss-triage-agent/internal/triage"
	"github.com/spf13/cobra"
)

var triageCmd = &cobra.Command{
	Use:   "triage",
	Short: "Run the AFK triage workflow for one or more issues",
	Long: `Assess GitHub issues and apply triage output automatically.

Without --issue, lists open issues that are unlabeled or carry needs-triage.
With --issue, runs OpenCode with the embedded triage skill, posts a comment
prefixed with the AI triage disclaimer, and swaps category/state labels.

Requires GITHUB_TOKEN and opencode on PATH. Resolves the GitHub repository
from remote.origin.url in --repo (default: current directory).`,
	Example: `  # List candidates needing triage
  oss-triage-agent triage

  # Triage a single issue
  oss-triage-agent triage --issue 42
  oss-triage-agent triage --issue 42 --repo /path/to/target-repo`,
	RunE: runTriage,
}

type triageOptions struct {
	IssueID     string
	IdleTimeout time.Duration
}

type triageRuntimeOptions struct {
	triageOptions
	RepoPath                     string
	Model                        string
	Variant                      string
	AgentName                    string
	SandboxMode                  string
	DangerouslySkipPermissions   bool
}

type triageWorkflowDeps struct {
	Issues   issuepkg.IssueTracker
	Sandbox  sandbox.SandboxProvider
	RepoRoot string
	Agent    agent.AgentProvider
	Prompt   prompt.Builder
	Log      *logging.CharmLogger
}

var (
	triageDepsResolver   = resolveTriageWorkflowDeps
	triageWorkflowRunner = runTriageWorkflow
	triageLister         = listTriageCandidates
)

func runTriage(cmd *cobra.Command, args []string) error {
	issueID, listErr := resolveIssue(args)
	if listErr != nil {
		return runTriageList(cmd)
	}

	opts := triageRuntimeOptions{
		triageOptions: triageOptions{
			IssueID:     issueID,
			IdleTimeout: triageIdleTimeout,
		},
		RepoPath:                   triageRepoPath,
		Model:                      triageModel,
		Variant:                    triageVariant,
		AgentName:                  triageAgentName,
		SandboxMode:                triageSandboxMode,
		DangerouslySkipPermissions: triageDangerouslySkipPermissions,
	}

	deps, err := triageDepsResolver(opts)
	if err != nil {
		return err
	}
	deps.Log = logging.NewCharm(cmd.ErrOrStderr(), "triage")

	summary, err := triageWorkflowRunner(cmd.Context(), deps, opts.triageOptions)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"triage completed for issue #%d with %s in %s\n",
		summary.IssueNumber,
		summary.AgentName,
		summary.SandboxKind,
	)
	return nil
}

func runTriageList(cmd *cobra.Command) error {
	opts := triageRuntimeOptions{
		RepoPath: triageRepoPath,
	}
	deps, err := triageDepsResolver(opts)
	if err != nil {
		return err
	}

	candidates, err := triageLister(cmd.Context(), deps.Issues)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No issues need triage.")
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Issues needing triage (%d):\n\n", len(candidates))
	for _, c := range candidates {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  [%s]\n", c.Number, c.Title, c.Bucket)
	}
	return nil
}

type triageCandidate struct {
	issuepkg.Issue
	Bucket string
}

func listTriageCandidates(ctx context.Context, tracker issuepkg.IssueTracker) ([]triageCandidate, error) {
	needsTriage, err := tracker.ListIssues(ctx, "label:needs-triage")
	if err != nil {
		return nil, fmt.Errorf("list needs-triage issues: %w", err)
	}
	allOpen, err := tracker.ListIssues(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list open issues: %w", err)
	}

	byNumber := make(map[int]triageCandidate)
	for _, it := range needsTriage {
		byNumber[it.Number] = triageCandidate{Issue: it, Bucket: "needs-triage"}
	}
	for _, it := range allOpen {
		if isUntriaged(it) {
			byNumber[it.Number] = triageCandidate{Issue: it, Bucket: "unlabeled"}
		}
	}

	out := make([]triageCandidate, 0, len(byNumber))
	for _, c := range byNumber {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func isUntriaged(it issuepkg.Issue) bool {
	hasCategory := false
	hasState := false
	for _, label := range it.Labels {
		if triagepkg.IsCategoryLabel(label) {
			hasCategory = true
		}
		if triagepkg.IsStateLabel(label) {
			hasState = true
		}
	}
	return !hasCategory && !hasState
}

func runTriageWorkflow(ctx context.Context, deps triageWorkflowDeps, opts triageOptions) (summary orchestrator.RunSummary, err error) {
	log := deps.Log.Charm()
	log.Info("reading issue", "issue_id", opts.IssueID)
	it, err := deps.Issues.ReadIssue(ctx, opts.IssueID)
	if err != nil {
		return orchestrator.RunSummary{}, fmt.Errorf("read issue: %w", err)
	}
	log.Info("loaded issue", "number", it.Number, "title", singleLine(it.Title))

	o := orchestrator.New(orchestrator.Deps{
		Agent:   deps.Agent,
		Sandbox: deps.Sandbox,
		Issues:  deps.Issues,
	})
	log.Info(
		"starting agent",
		"agent", deps.Agent.Name(),
		"workspace", deps.RepoRoot,
		"idle_timeout", opts.IdleTimeout,
	)
	summary, err = o.Run(ctx, orchestrator.RunInput{
		IssueID:     opts.IssueID,
		Issue:       it,
		Prompt:      deps.Prompt.ForTriage(*it),
		Workspace:   deps.RepoRoot,
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
	if err != nil {
		return summary, err
	}

	rawOutput := agentOutputText(summary.Events)
	commentBody, result, parseErr := triagepkg.ParseAgentOutput(rawOutput)
	if parseErr != nil {
		return summary, fmt.Errorf("parse triage output: %w", parseErr)
	}

	log.Info("applying labels", "category", result.Category, "state", result.State)
	if labelErr := applyTriageLabels(ctx, deps.Issues, opts.IssueID, *it, result); labelErr != nil {
		return summary, labelErr
	}

	log.Info("posting issue comment", "number", it.Number)
	if commentErr := deps.Issues.Comment(ctx, opts.IssueID, commentBody); commentErr != nil {
		return summary, fmt.Errorf("comment on issue: %w", commentErr)
	}
	if result.Close {
		log.Info("close requested but issue close is not implemented yet", "number", it.Number)
	}
	return summary, nil
}

func agentOutputText(events []agent.AgentEvent) string {
	var b strings.Builder
	for _, ev := range events {
		switch ev.Kind {
		case agent.EventText:
			b.WriteString(ev.Text)
		case agent.EventResult:
			if ev.Result != nil {
				b.WriteString(ev.Result.Output)
			}
		}
	}
	return b.String()
}

func applyTriageLabels(ctx context.Context, tracker issuepkg.IssueTracker, id string, it issuepkg.Issue, result triagepkg.Result) error {
	for _, label := range it.Labels {
		if !triagepkg.IsCategoryLabel(label) && !triagepkg.IsStateLabel(label) {
			continue
		}
		if err := tracker.RemoveLabel(ctx, id, label); err != nil {
			return fmt.Errorf("remove label %q: %w", label, err)
		}
	}
	if err := tracker.AddLabel(ctx, id, result.Category); err != nil {
		return fmt.Errorf("add category label: %w", err)
	}
	if err := tracker.AddLabel(ctx, id, result.State); err != nil {
		return fmt.Errorf("add state label: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(triageCmd)
}
