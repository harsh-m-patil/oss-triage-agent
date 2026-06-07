package cmd

import (
	"reflect"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/pi"
)

func TestResolveWorkflowAgent_selectsOpenCode(t *testing.T) {
	t.Parallel()

	got, err := resolveWorkflowAgent(workflowAgentConfig{
		Provider: "opencode",
		Model:    "opencode/big-pickle",
		Variant:  "high",
	})
	if err != nil {
		t.Fatalf("resolveWorkflowAgent: %v", err)
	}
	p, ok := got.(*opencode.Provider)
	if !ok {
		t.Fatalf("provider type = %T, want *opencode.Provider", got)
	}
	launch := p.BuildLaunch("hello")
	want := agent.Launch{
		Argv: []string{
			"opencode", "run", "--format", "json",
			"--model", "opencode/big-pickle",
			"--variant", "high",
			"hello",
		},
	}
	if !reflect.DeepEqual(launch, want) {
		t.Fatalf("BuildLaunch() = %+v, want %+v", launch, want)
	}
}

func TestResolveWorkflowAgent_selectsPi(t *testing.T) {
	t.Parallel()

	got, err := resolveWorkflowAgent(workflowAgentConfig{
		Provider: "pi",
		Model:    "claude-sonnet-4",
		Thinking: "medium",
		Session:  "sess-1",
	})
	if err != nil {
		t.Fatalf("resolveWorkflowAgent: %v", err)
	}
	p, ok := got.(*pi.Provider)
	if !ok {
		t.Fatalf("provider type = %T, want *pi.Provider", got)
	}
	launch := p.BuildLaunch("hello")
	want := agent.Launch{
		Argv: []string{
			"pi", "-p", "--mode", "json",
			"--model", "claude-sonnet-4",
			"--thinking", "medium",
			"--session", "sess-1",
		},
		Stdin: "hello",
	}
	if !reflect.DeepEqual(launch, want) {
		t.Fatalf("BuildLaunch() = %+v, want %+v", launch, want)
	}
}

func TestResolveWorkflowAgent_rejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := resolveWorkflowAgent(workflowAgentConfig{Provider: "unknown"})
	if err == nil {
		t.Fatal("resolveWorkflowAgent: want error, got nil")
	}
}

func TestWorkflowAgentBinary(t *testing.T) {
	t.Parallel()

	if got := workflowAgentBinary("pi"); got != "pi" {
		t.Fatalf("workflowAgentBinary(pi) = %q, want pi", got)
	}
	if got := workflowAgentBinary("opencode"); got != "opencode" {
		t.Fatalf("workflowAgentBinary(opencode) = %q, want opencode", got)
	}
}
