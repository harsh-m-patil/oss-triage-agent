package orchestrator_test

import (
	"context"
	"testing"

	agentfake "github.com/harsh-m-patil/oss-triage-agent/internal/agent/fake"
	issuefake "github.com/harsh-m-patil/oss-triage-agent/internal/issue/fake"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	sandboxfake "github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/fake"
)

func TestRun_triagesIssueUsingProviderInterfacesOnly(t *testing.T) {
	t.Parallel()

	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"42": {Number: 42, Title: "skeleton", Body: "implement internal packages"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   agentfake.NewProvider(),
		Sandbox: sandboxfake.NewProvider(sandbox.SandboxNone),
		Issues:  tracker,
	})

	summary, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:   "42",
		Workspace: "/tmp/ws",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.IssueNumber != 42 {
		t.Fatalf("IssueNumber = %d, want 42", summary.IssueNumber)
	}
	if summary.AgentName != "fake" {
		t.Fatalf("AgentName = %q, want fake", summary.AgentName)
	}
	if summary.SandboxKind != sandbox.SandboxNone {
		t.Fatalf("SandboxKind = %q, want none", summary.SandboxKind)
	}
}
