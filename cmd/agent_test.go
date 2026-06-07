package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
)

type echoJSONProvider struct{}

func (echoJSONProvider) Name() string { return "echo-json" }
func (echoJSONProvider) Env() map[string]string { return nil }
func (echoJSONProvider) BuildLaunch(prompt string) agent.Launch {
	line := fmt.Sprintf(`{"type":"text","content":%q}`, prompt)
	return agent.Launch{Argv: []string{"echo", line}}
}
func (echoJSONProvider) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	var raw struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}
	if raw.Type != "text" {
		return nil, nil
	}
	return []agent.AgentEvent{{Kind: agent.EventText, Text: raw.Content}}, nil
}

func TestStreamAgentEvents_printsNormalizedEvents(t *testing.T) {
	t.Parallel()

	provider := echoJSONProvider{}
	var out bytes.Buffer
	err := streamAgentEvents(context.Background(), provider, "hello", &out, io.Discard)
	if err != nil {
		t.Fatalf("streamAgentEvents: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("output lines = %d, want 1", len(lines))
	}

	var event agent.AgentEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Kind != agent.EventText || event.Text != "hello" {
		t.Fatalf("event = %+v, want kind=text text=hello", event)
	}
	if strings.Contains(lines[0], `"Kind"`) {
		t.Fatalf("output uses PascalCase JSON keys: %s", lines[0])
	}
}

type stderrFloodProvider struct{}

func (stderrFloodProvider) Name() string { return "stderr-flood" }
func (stderrFloodProvider) Env() map[string]string { return nil }
func (stderrFloodProvider) BuildLaunch(prompt string) agent.Launch {
	script := fmt.Sprintf(
		`i=0; while [ $i -lt 5000 ]; do echo "stderr-$i" >&2; i=$((i+1)); done; echo '{"type":"text","content":%q}'`,
		prompt,
	)
	return agent.Launch{Argv: []string{"sh", "-c", script}}
}
func (stderrFloodProvider) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	return echoJSONProvider{}.ParseStreamLine(line)
}

func TestStreamAgentEvents_drainsStderrWhileReadingStdout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var out, errOut bytes.Buffer
	err := streamAgentEvents(ctx, stderrFloodProvider{}, "ok", &out, &errOut)
	if err != nil {
		t.Fatalf("streamAgentEvents: %v", err)
	}
	if !strings.Contains(out.String(), `"text":"ok"`) {
		t.Fatalf("stdout = %q, want normalized text event", out.String())
	}
	if errOut.Len() == 0 {
		t.Fatal("stderr was not drained")
	}
}

type stdinEchoProvider struct{}

func (stdinEchoProvider) Name() string { return "stdin-echo" }
func (stdinEchoProvider) Env() map[string]string { return nil }
func (stdinEchoProvider) BuildLaunch(prompt string) agent.Launch {
	return agent.Launch{Argv: []string{"cat"}, Stdin: prompt}
}
func (stdinEchoProvider) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	return []agent.AgentEvent{{Kind: agent.EventText, Text: line}}, nil
}

func TestStreamAgentEvents_deliversStdinToCommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := streamAgentEvents(context.Background(), stdinEchoProvider{}, "stdin hello", &out, io.Discard)
	if err != nil {
		t.Fatalf("streamAgentEvents: %v", err)
	}
	if !strings.Contains(out.String(), `"text":"stdin hello"`) {
		t.Fatalf("stdout = %q, want normalized stdin prompt event", out.String())
	}
}

func TestResolveAgentProvider_selectsPi(t *testing.T) {
	t.Parallel()

	agentProvider = "pi"
	agentModel = "claude-sonnet-4"
	agentThinking = "high"
	agentSession = "sess-1"
	t.Cleanup(func() {
		agentProvider = "opencode"
		agentModel = "opencode/big-pickle"
		agentThinking = ""
		agentSession = ""
	})

	provider, binary, err := resolveAgentProvider("pi")
	if err != nil {
		t.Fatalf("resolveAgentProvider: %v", err)
	}
	if binary != "pi" {
		t.Fatalf("binary = %q, want pi", binary)
	}
	launch := provider.BuildLaunch("hello")
	want := agent.Launch{
		Argv: []string{
			"pi", "-p", "--mode", "json",
			"--model", "claude-sonnet-4",
			"--thinking", "high",
			"--session", "sess-1",
		},
		Stdin: "hello",
	}
	if !reflect.DeepEqual(launch, want) {
		t.Fatalf("BuildLaunch() = %+v, want %+v", launch, want)
	}
}

func TestResolveAgentPrompt_prefersFlag(t *testing.T) {
	t.Parallel()

	got, err := resolveAgentPrompt("from flag")
	if err != nil {
		t.Fatalf("resolveAgentPrompt: %v", err)
	}
	if got != "from flag" {
		t.Fatalf("prompt = %q, want from flag", got)
	}
}
