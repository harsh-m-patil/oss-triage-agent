package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	IssueID     string              `json:"issue_id"`
	Issue       *issue.Issue        `json:"-"`
	Prompt      string              `json:"prompt,omitempty"`
	Workspace   string              `json:"workspace"`
	IdleTimeout time.Duration       `json:"idle_timeout,omitempty"`
	Progress    func(ProgressEvent) `json:"-"`
}

// RunSummary captures observable outcomes from a triage run.
type RunSummary struct {
	IssueNumber int                 `json:"issue_number"`
	AgentName   string              `json:"agent_name"`
	SandboxKind sandbox.SandboxKind `json:"sandbox_kind"`
	// Completed is true when the agent process exits cleanly.
	Completed bool `json:"completed"`
	// Success is true when the run finished without timeout or process error.
	Success bool `json:"success"`
	// TimeoutKind is set when a configured timeout ended the run unsuccessfully.
	TimeoutKind TimeoutKind `json:"timeout_kind,omitempty"`
	// Events holds normalized agent stream events from stdout lines.
	Events []agent.AgentEvent `json:"events,omitempty"`
}

// ProgressKind identifies a live orchestrator progress update.
type ProgressKind string

const (
	ProgressAgentStart  ProgressKind = "agent_start"
	ProgressAgentEvent  ProgressKind = "agent_event"
	ProgressAgentStderr ProgressKind = "agent_stderr"
	ProgressHeartbeat   ProgressKind = "heartbeat"
)

// ProgressEvent is a live update emitted while an orchestrator run is active.
type ProgressEvent struct {
	Kind       ProgressKind
	Command    string
	Args       []string
	Event      *agent.AgentEvent
	StderrLine string
	Wait       time.Duration
	Completed  bool
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
	it := in.Issue
	if it == nil {
		var err error
		it, err = o.deps.Issues.ReadIssue(ctx, in.IssueID)
		if err != nil {
			return RunSummary{}, fmt.Errorf("read issue: %w", err)
		}
	}

	handle, err := o.deps.Sandbox.Create(ctx, in.Workspace)
	if err != nil {
		return RunSummary{}, fmt.Errorf("create sandbox: %w", err)
	}
	defer handle.Close()

	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = it.Body
	}
	launch := o.deps.Agent.BuildLaunch(prompt)
	if len(launch.Argv) == 0 {
		return RunSummary{}, fmt.Errorf("agent %q returned empty command", o.deps.Agent.Name())
	}

	summary := RunSummary{
		IssueNumber: it.Number,
		AgentName:   o.deps.Agent.Name(),
		SandboxKind: handle.Kind(),
	}

	err = o.runAgent(
		ctx,
		handle,
		launch.Argv[0],
		launch.Argv[1:],
		launch.Stdin,
		o.deps.Agent.Env(),
		in.IdleTimeout,
		in.Progress,
		&summary,
	)
	return summary, err
}
