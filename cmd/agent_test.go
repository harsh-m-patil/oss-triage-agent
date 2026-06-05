package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
)

type echoJSONProvider struct{}

func (echoJSONProvider) Name() string { return "echo-json" }
func (echoJSONProvider) Env() map[string]string { return nil }
func (echoJSONProvider) BuildCommand(prompt string) []string {
	line := fmt.Sprintf(`{"type":"text","content":%q}`, prompt)
	return []string{"echo", line}
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
	err := streamAgentEvents(context.Background(), provider, "hello", &out)
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
		t.Fatalf("event = %+v, want text hello", event)
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
