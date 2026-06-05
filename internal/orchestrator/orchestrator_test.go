package orchestrator_test

import (
	"context"
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
		`test "$MARKER" = "value" && printf '%s\n' '` + orchestrator.CompletionSignal + `'`,
	}
}

func (envFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

func TestRun_succeedsWhenStdoutContainsCompletionSignal(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Title: "afk", Body: "do work"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &signalFixtureAgent{},
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
		t.Fatal("Completed = false, want true (completion signal seen)")
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

// signalFixtureAgent runs a shell script that emits the AFK completion token on stdout.
type signalFixtureAgent struct{}

func (signalFixtureAgent) Name() string { return "fixture" }

func (signalFixtureAgent) Env() map[string]string { return nil }

func (signalFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`printf '%s\n' '` + orchestrator.CompletionSignal + `'`,
	}
}

func (signalFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
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
		`printf '%s\n' '{"type":"text","content":"hi"}' '{"type":"text","content":"bye"}'` +
			`; printf '%s\n' '` + orchestrator.CompletionSignal + `'`,
	}
}

func (a *jsonlFixtureAgent) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	if line == orchestrator.CompletionSignal {
		return nil, nil
	}
	a.parsedLines++
	return []agent.AgentEvent{{Kind: agent.EventText, Text: line}}, nil
}

func TestRun_failsWhenIdleTimeoutExpiresBeforeCompletionSignal(t *testing.T) {
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

func TestRun_succeedsWhenCompletionTimeoutExpiresAfterSignalDespiteHungProcess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tracker := issuefake.NewTracker(map[string]issuefake.Issue{
		"1": {Number: 1, Body: "hang"},
	})
	o := orchestrator.New(orchestrator.Deps{
		Agent:   &hangAfterSignalFixtureAgent{},
		Sandbox: nosandbox.NewProvider(),
		Issues:  tracker,
	})

	summary, err := o.Run(context.Background(), orchestrator.RunInput{
		IssueID:           "1",
		Workspace:         workspace,
		IdleTimeout:       5 * time.Second,
		CompletionTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !summary.Completed || !summary.Success {
		t.Fatalf("Completed=%v Success=%v, want both true", summary.Completed, summary.Success)
	}
}

type hangAfterSignalFixtureAgent struct{}

func (hangAfterSignalFixtureAgent) Name() string { return "hang-after-signal" }

func (hangAfterSignalFixtureAgent) Env() map[string]string { return nil }

func (hangAfterSignalFixtureAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`printf '%s\n' '` + orchestrator.CompletionSignal + `'; sleep 3600`,
	}
}

func (hangAfterSignalFixtureAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}
