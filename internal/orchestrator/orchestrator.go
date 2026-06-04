package orchestrator

import (
	"context"
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

// Deps are the provider interfaces required to run a workflow step.
type Deps struct {
	Agent   agent.AgentProvider
	Sandbox sandbox.SandboxProvider
	Issues  issue.IssueTracker
}

// RunInput identifies the issue and workspace for a triage run.
type RunInput struct {
	IssueID   string
	Workspace string
}

// RunSummary captures observable outcomes from a triage run.
type RunSummary struct {
	IssueNumber int
	AgentName   string
	SandboxKind sandbox.SandboxKind
}

// Orchestrator coordinates AFK workflows using provider interfaces only.
type Orchestrator struct {
	deps Deps
}

// New returns an orchestrator backed by the given dependencies.
func New(deps Deps) *Orchestrator {
	return &Orchestrator{deps: deps}
}

// Run loads the issue, prepares a sandbox, and builds the agent command.
func (o *Orchestrator) Run(ctx context.Context, in RunInput) (RunSummary, error) {
	it, err := o.deps.Issues.ReadIssue(ctx, in.IssueID)
	if err != nil {
		return RunSummary{}, fmt.Errorf("read issue: %w", err)
	}

	handle, err := o.deps.Sandbox.Create(ctx, in.Workspace)
	if err != nil {
		return RunSummary{}, fmt.Errorf("create sandbox: %w", err)
	}
	defer handle.Close()

	_ = o.deps.Agent.BuildCommand(it.Body)

	return RunSummary{
		IssueNumber: it.Number,
		AgentName:   o.deps.Agent.Name(),
		SandboxKind: handle.Kind(),
	}, nil
}
