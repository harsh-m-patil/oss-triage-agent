// Package pi implements agent.AgentProvider for the Pi CLI.
//
// Expected environment variables (set via Options.Env):
//   - ANTHROPIC_API_KEY — API key when required by the configured model/provider.
package pi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
)

var _ agent.AgentProvider = (*Provider)(nil)

// Options configures a Pi provider instance.
type Options struct {
	Thinking      string
	ResumeSession string
	Env           map[string]string
}

// Provider launches pi -p --mode json and normalizes its JSONL stdout.
type Provider struct {
	model string
	opts  Options
}

// NewProvider returns a Pi agent provider for the given model.
func NewProvider(model string, opts Options) *Provider {
	return &Provider{model: model, opts: opts}
}

func (p *Provider) Name() string { return "pi" }

func (p *Provider) Env() map[string]string {
	if len(p.opts.Env) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(p.opts.Env))
	for k, v := range p.opts.Env {
		out[k] = v
	}
	return out
}

func (p *Provider) BuildLaunch(prompt string) agent.Launch {
	args := []string{"pi", "-p", "--mode", "json", "--model", p.model}
	if p.opts.Thinking != "" {
		args = append(args, "--thinking", p.opts.Thinking)
	}
	if p.opts.ResumeSession != "" {
		args = append(args, "--session", p.opts.ResumeSession)
	}
	return agent.Launch{Argv: args, Stdin: prompt}
}

func (p *Provider) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil, nil
	}

	var raw streamEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse stream line: %w", err)
	}

	switch raw.Type {
	case "session":
		if raw.ID == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventSessionID, SessionID: raw.ID}}, nil

	case "message_update":
		if raw.AssistantMessageEvent == nil || raw.AssistantMessageEvent.Type != "text_delta" {
			return nil, nil
		}
		if raw.AssistantMessageEvent.Delta == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventText, Text: raw.AssistantMessageEvent.Delta}}, nil

	case "tool_execution_start":
		args, ok := toolArgs(raw.ToolName, raw.Args)
		if !ok {
			return nil, nil
		}
		return []agent.AgentEvent{{
			Kind:     agent.EventToolCall,
			ToolCall: &agent.ToolCall{Name: raw.ToolName, Args: args},
		}}, nil

	case "agent_error", "error":
		msg := jsonStringField(raw.Message)
		if msg == "" {
			msg = strings.TrimSpace(raw.Error)
		}
		if msg == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventResult, Result: &agent.Result{Output: msg}}}, nil

	case "message_end":
		return parseMessageEnd(raw.Message)

	case "agent_end":
		text := lastAssistantText(raw.Messages)
		if text == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventResult, Result: &agent.Result{Output: text}}}, nil

	default:
		return nil, nil
	}
}

func parseMessageEnd(raw json.RawMessage) ([]agent.AgentEvent, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var msg endMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("parse stream line: %w", err)
	}
	if msg.Role != "assistant" || msg.Usage == nil {
		return nil, nil
	}
	if msg.Usage.Input == 0 && msg.Usage.Output == 0 {
		return nil, nil
	}
	return []agent.AgentEvent{{
		Kind: agent.EventUsage,
		Usage: &agent.Usage{
			InputTokens:  msg.Usage.Input,
			OutputTokens: msg.Usage.Output,
		},
	}}, nil
}

func toolArgs(tool string, input map[string]json.RawMessage) (string, bool) {
	if input == nil {
		return "", false
	}
	switch tool {
	case "bash":
		if v, ok := stringField(input, "command"); ok {
			return v, true
		}
	case "read":
		if v, ok := stringField(input, "path"); ok {
			return v, true
		}
	case "write", "edit":
		if v, ok := stringField(input, "path"); ok {
			return v, true
		}
	}
	b, err := json.Marshal(input)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func stringField(input map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := input[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return "", false
	}
	return s, true
}

func jsonStringField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func lastAssistantText(messages []agentMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "assistant" {
			continue
		}
		var parts []string
		for _, block := range messages[i].Content {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
	}
	return ""
}

type streamEvent struct {
	Type                  string                 `json:"type"`
	ID                    string                 `json:"id"`
	AssistantMessageEvent *assistantMessageEvent `json:"assistantMessageEvent"`
	ToolName              string                 `json:"toolName"`
	Args                  map[string]json.RawMessage `json:"args"`
	Messages              []agentMessage         `json:"messages"`
	Error                 string                 `json:"error"`
	Message               json.RawMessage        `json:"message"`
}

type assistantMessageEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

type agentMessage struct {
	Role    string          `json:"role"`
	Content []contentBlock  `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type endMessage struct {
	Role  string   `json:"role"`
	Usage *piUsage `json:"usage"`
}

type piUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}
