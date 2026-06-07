package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/spf13/cobra"
)

func TestParseGitHubRepoRemote_supportsCommonGitHubURLForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		owner string
		repo  string
	}{
		{
			name:  "https",
			raw:   "https://github.com/harsh-m-patil/oss-triage-agent.git",
			owner: "harsh-m-patil",
			repo:  "oss-triage-agent",
		},
		{
			name:  "ssh scp style",
			raw:   "git@github.com:harsh-m-patil/oss-triage-agent.git",
			owner: "harsh-m-patil",
			repo:  "oss-triage-agent",
		},
		{
			name:  "ssh url",
			raw:   "ssh://git@github.com/harsh-m-patil/oss-triage-agent.git",
			owner: "harsh-m-patil",
			repo:  "oss-triage-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			owner, repo, err := parseGitHubRepoRemote(tt.raw)
			if err != nil {
				t.Fatalf("parseGitHubRepoRemote: %v", err)
			}
			if owner != tt.owner || repo != tt.repo {
				t.Fatalf("owner/repo = %s/%s, want %s/%s", owner, repo, tt.owner, tt.repo)
			}
		})
	}
}

func TestNewBuildSandboxProvider_rejectsUnknownMode(t *testing.T) {
	t.Parallel()

	_, err := newBuildSandboxProvider("weird")
	if err == nil {
		t.Fatal("newBuildSandboxProvider: want error, got nil")
	}
}

func TestRunBuild_passesConfiguredFlagsToResolverAndWorkflow(t *testing.T) {
	t.Parallel()

	restore := snapshotBuildGlobals()
	defer restore()

	buildRepoPath = "/repo"
	buildProvider = workflowProviderPi
	buildModel = "claude-sonnet-4"
	buildVariant = "variant-a"
	buildAgentName = "builder"
	buildThinking = "high"
	buildSession = "sess-abc"
	buildSandboxMode = buildSandboxNoSandbox
	buildDangerouslySkipPermissions = true
	buildIdleTimeout = 2 * time.Minute
	issue = ""

	var gotResolve buildRuntimeOptions
	var gotRun buildOptions
	var gotLogSet bool
	buildDepsResolver = func(opts buildRuntimeOptions) (buildWorkflowDeps, error) {
		gotResolve = opts
		return buildWorkflowDeps{}, nil
	}
	buildWorkflowRunner = func(_ context.Context, deps buildWorkflowDeps, opts buildOptions) (orchestrator.RunSummary, error) {
		gotRun = opts
		gotLogSet = deps.Log != nil
		return orchestrator.RunSummary{
			IssueNumber: 9,
			AgentName:   "opencode",
			SandboxKind: "none",
		}, nil
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	if err := runBuild(cmd, []string{"9"}); err != nil {
		t.Fatalf("runBuild: %v", err)
	}

	if gotResolve.IssueID != "9" {
		t.Fatalf("resolver IssueID = %q, want 9", gotResolve.IssueID)
	}
	if gotResolve.RepoPath != "/repo" || gotResolve.Provider != workflowProviderPi || gotResolve.Model != "claude-sonnet-4" {
		t.Fatalf("resolver opts = %+v", gotResolve)
	}
	if gotResolve.Thinking != "high" || gotResolve.Session != "sess-abc" {
		t.Fatalf("resolver pi opts = %+v", gotResolve)
	}
	if gotResolve.SandboxMode != buildSandboxNoSandbox || !gotResolve.DangerouslySkipPermissions {
		t.Fatalf("resolver opts = %+v", gotResolve)
	}
	if gotRun.IssueID != "9" || gotRun.IdleTimeout != 2*time.Minute {
		t.Fatalf("runner opts = %+v", gotRun)
	}
	if !gotLogSet {
		t.Fatal("runner deps.Log was nil, want stderr writer")
	}
	if !strings.Contains(out.String(), "build completed for issue #9") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func snapshotBuildGlobals() func() {
	prevRepoPath := buildRepoPath
	prevProvider := buildProvider
	prevModel := buildModel
	prevVariant := buildVariant
	prevAgentName := buildAgentName
	prevThinking := buildThinking
	prevSession := buildSession
	prevSandboxMode := buildSandboxMode
	prevSkipPermissions := buildDangerouslySkipPermissions
	prevIdleTimeout := buildIdleTimeout
	prevIssue := issue
	prevResolver := buildDepsResolver
	prevRunner := buildWorkflowRunner

	return func() {
		buildRepoPath = prevRepoPath
		buildProvider = prevProvider
		buildModel = prevModel
		buildVariant = prevVariant
		buildAgentName = prevAgentName
		buildThinking = prevThinking
		buildSession = prevSession
		buildSandboxMode = prevSandboxMode
		buildDangerouslySkipPermissions = prevSkipPermissions
		buildIdleTimeout = prevIdleTimeout
		issue = prevIssue
		buildDepsResolver = prevResolver
		buildWorkflowRunner = prevRunner
	}
}
