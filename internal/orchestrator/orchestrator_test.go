package orchestrator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	issuefake "github.com/harsh-m-patil/oss-triage-agent/internal/issue/fake"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/nosandbox"
)

func TestRun_appliesAgentEnvInSandboxExec(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Body: "check env"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &envFixtureAgent{},
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	_, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:   "1",
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

type envFixtureAgent struct{}

func (envFixtureAgent) Name() string { return "env-fixture" }

func (envFixtureAgent) Env() map[string]string {
	return map[string]string{"MARKER": "value"}
}

func (envFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`test "$MARKER" = "value"`,
	}
}

func (envFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

func TestRun_succeedsWhenAgentExitsCleanly(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Title: "afk", Body: "do work"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &cleanExitFixtureAgent{},
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	summary, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:   "1",
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !summary.Completed {
		t.Fatal("Completed = false, want true")
	}
	if !summary.Success {
		t.Fatal("Success = false, want true")
	}
	if summary.IssueNumber != 1 {
		t.Fatalf("IssueNumber = %d, want 1", summary.IssueNumber)
	}
	if summary.AgentName != "fixture" {
		t.Fatalf("AgentName = %q, want fixture", summary.AgentName)
	}
	if summary.SandboxKind != sandbox.SandboxNone {
		t.Fatalf("SandboxKind = %q, want none", summary.SandboxKind)
	}
}

type cleanExitFixtureAgent struct{}

func (cleanExitFixtureAgent) Name() string { return "fixture" }

func (cleanExitFixtureAgent) Env() map[string]string { return nil }

func (cleanExitFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`printf '%s\n' 'done'`,
	}
}

func (cleanExitFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

func TestRun_parsesEachStdoutLineThroughAgentProvider(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Body: "prompt"},
	})
	agent := &jsonlFixtureAgent{}
	o := orchestrator.New(orchestrator.Deps{
		Agent:   agent,
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	_, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:   "1",
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if agent.parsedLines != 2 {
		t.Fatalf("parsedLines = %d, want 2", agent.parsedLines)
	}
}

type jsonlFixtureAgent struct {
	parsedLines int
}

func (a *jsonlFixtureAgent) Name() string { return "jsonl-fixture" }

func (a *jsonlFixtureAgent) Env() map[string]string { return nil }

func (a *jsonlFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`printf '%s\n' '{"type":"text","content":"hi"}' '{"type":"text","content":"bye"}'`,
	}
}

func (a *jsonlFixtureAgent) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	a.parsedLines++
	return []agent.AgentEvent{{Kind: agent.EventText, Text: line}}, nil
}

func TestRun_failsWhenIdleTimeoutExpiresBeforeAgentExit(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Body: "wait"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &silentFixtureAgent{},
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	summary, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:     "1",
		Workspace:   workspace,
		IdleTimeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Run: want error on idle timeout, got nil")
	}
	if summary.Success {
		t.Fatal("Success = true, want false")
	}
	if summary.TimeoutKind != orchestrator.TimeoutIdle {
		t.Fatalf("TimeoutKind = %q, want %q", summary.TimeoutKind, orchestrator.TimeoutIdle)
	}
}

type silentFixtureAgent struct{}

func (silentFixtureAgent) Name() string { return "silent" }

func (silentFixtureAgent) Env() map[string]string { return nil }

func (silentFixtureAgent) BuildCommand(string) []string {
	return []string{"sleep", "3600"}
}

func (silentFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

func TestRun_includesStderrInAgentExecError(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Body: "fail loudly"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &stderrFailureFixtureAgent{},
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	_, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:   "1",
		Workspace: workspace,
	})
	if err == nil {
		t.Fatal("Run: want error, got nil")
	}
	for _, want := range []string{"exit status 17", "permission denied", "opencode"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want substring %q", err, want)
		}
	}
}

type stderrFailureFixtureAgent struct{}

func (stderrFailureFixtureAgent) Name() string { return "opencode" }

func (stderrFailureFixtureAgent) Env() map[string]string { return nil }

func (stderrFailureFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`echo 'permission denied' >&2; exit 17`,
	}
}

func (stderrFailureFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}
