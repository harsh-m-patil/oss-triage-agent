package pi_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/pi"
)

func TestProvider_Name_returnsPi(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("claude-sonnet-4", pi.Options{})
	if got := p.Name(); got != "pi" {
		t.Fatalf("Name() = %q, want pi", got)
	}
}

func TestProvider_ParseStreamLine_fixture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		want    []agent.AgentEvent
	}{
		{
			fixture: "session_id.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventSessionID, SessionID: "ccd569e0-4e1b-4c7d-a981-637ed4107310"},
			},
		},
		{
			fixture: "text_delta.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventText, Text: "Hello"},
			},
		},
		{
			fixture: "tool_bash.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventToolCall, ToolCall: &agent.ToolCall{Name: "bash", Args: "npm test"}},
			},
		},
		{
			fixture: "error.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventResult, Result: &agent.Result{Output: "Invalid API key"}},
			},
		},
		{
			fixture: "agent_error.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventResult, Result: &agent.Result{Output: "Model not found"}},
			},
		},
		{
			fixture: "agent_end.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventResult, Result: &agent.Result{Output: "Done."}},
			},
		},
		{
			fixture: "usage.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventUsage, Usage: &agent.Usage{InputTokens: 1200, OutputTokens: 340}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()

			line := readFixture(t, tt.fixture)
			p := pi.NewProvider("claude-sonnet-4", pi.Options{})
			got, err := p.ParseStreamLine(line)
			if err != nil {
				t.Fatalf("ParseStreamLine: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("events = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestProvider_ParseStreamLine_nonJSONReturnsEmpty(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("model", pi.Options{})
	for _, line := range []string{"", "not json", "plain log line"} {
		events, err := p.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("ParseStreamLine(%q): %v", line, err)
		}
		if len(events) != 0 {
			t.Fatalf("ParseStreamLine(%q) = %+v, want empty", line, events)
		}
	}
}

func TestProvider_ParseStreamLine_malformedJSONReturnsError(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("model", pi.Options{})
	_, err := p.ParseStreamLine("{bad json")
	if err == nil {
		t.Fatal("ParseStreamLine: want error for malformed JSON")
	}
}

func TestProvider_BuildLaunch_usesStdinNotArgv(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("claude-sonnet-4", pi.Options{})
	got := p.BuildLaunch("do something")
	want := agent.Launch{
		Argv:  []string{"pi", "-p", "--mode", "json", "--model", "claude-sonnet-4"},
		Stdin: "do something",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildLaunch() = %+v, want %+v", got, want)
	}
}

func TestProvider_BuildLaunch_includesOptionalFlags(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("claude-sonnet-4", pi.Options{
		Thinking:      "high",
		ResumeSession: "sess-123",
	})
	got := p.BuildLaunch("do something")
	want := agent.Launch{
		Argv: []string{
			"pi", "-p", "--mode", "json",
			"--model", "claude-sonnet-4",
			"--thinking", "high",
			"--session", "sess-123",
		},
		Stdin: "do something",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildLaunch() = %+v, want %+v", got, want)
	}
}

func TestProvider_Env_returnsConfiguredVariables(t *testing.T) {
	t.Parallel()

	p := pi.NewProvider("claude-sonnet-4", pi.Options{
		Env: map[string]string{"ANTHROPIC_API_KEY": "sk-test"},
	})
	got := p.Env()
	want := map[string]string{"ANTHROPIC_API_KEY": "sk-test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Env() = %v, want %v", got, want)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}
