package opencode_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
)

func TestProvider_Name_returnsOpenCode(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("opencode/big-pickle", opencode.Options{})
	if got := p.Name(); got != "opencode" {
		t.Fatalf("Name() = %q, want opencode", got)
	}
}

func TestProvider_BuildCommand_includesModelAndPrompt(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("opencode/big-pickle", opencode.Options{})
	got := p.BuildCommand("do something")
	want := []string{
		"opencode", "run", "--format", "json",
		"--model", "opencode/big-pickle",
		"do something",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildCommand() = %v, want %v", got, want)
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
				{Kind: agent.EventSessionID, SessionID: "ses_19cb8236effe4lu1aSmQyzbeP2"},
			},
		},
		{
			fixture: "text.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventText, Text: "Hello world COMPLETE "},
				{Kind: agent.EventResult, Result: &agent.Result{Output: "Hello world COMPLETE "}},
			},
		},
		{
			fixture: "tool_bash.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventToolCall, ToolCall: &agent.ToolCall{Name: "bash", Args: "npm test"}},
			},
		},
		{
			fixture: "usage.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventUsage, Usage: &agent.Usage{InputTokens: 1200, OutputTokens: 340}},
			},
		},
		{
			fixture: "error.jsonl",
			want: []agent.AgentEvent{
				{Kind: agent.EventResult, Result: &agent.Result{Output: "Invalid API key"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()

			line := readFixture(t, tt.fixture)
			p := opencode.NewProvider("opencode/big-pickle", opencode.Options{})
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

func TestProvider_BuildCommand_includesOptionalFlags(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("opencode/big-pickle", opencode.Options{
		Variant:                    "high",
		Agent:                      "build",
		DangerouslySkipPermissions: true,
	})
	got := p.BuildCommand("do something")
	want := []string{
		"opencode", "run", "--format", "json",
		"--model", "opencode/big-pickle",
		"--variant", "high",
		"--agent", "build",
		"--dangerously-skip-permissions",
		"do something",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildCommand() = %v, want %v", got, want)
	}
}

func TestProvider_Env_returnsConfiguredVariables(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("opencode/big-pickle", opencode.Options{
		Env: map[string]string{"OPENCODE_API_KEY": "sk-test"},
	})
	got := p.Env()
	want := map[string]string{"OPENCODE_API_KEY": "sk-test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Env() = %v, want %v", got, want)
	}
}

func TestProvider_ParseStreamLine_nonJSONReturnsEmpty(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("model", opencode.Options{})
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

	p := opencode.NewProvider("model", opencode.Options{})
	_, err := p.ParseStreamLine("{bad json")
	if err == nil {
		t.Fatal("ParseStreamLine: want error for malformed JSON")
	}
}

func TestProvider_ParseStreamLine_skipsIncompleteEvents(t *testing.T) {
	t.Parallel()

	p := opencode.NewProvider("model", opencode.Options{})

	tests := []struct {
		name string
		line string
	}{
		{
			name: "step_start without session",
			line: `{"type":"step_start","part":{"type":"step-start"}}`,
		},
		{
			name: "tool_use not completed",
			line: `{"type":"tool_use","part":{"type":"tool","tool":"bash","state":{"status":"running","input":{"command":"npm test"}}}}`,
		},
		{
			name: "step_finish without token counts",
			line: `{"type":"step_finish","part":{"type":"step-finish","tokens":{"total":100}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			events, err := p.ParseStreamLine(tt.line)
			if err != nil {
				t.Fatalf("ParseStreamLine: %v", err)
			}
			if len(events) != 0 {
				t.Fatalf("events = %+v, want empty", events)
			}
		})
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
