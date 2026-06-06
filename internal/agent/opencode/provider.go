// Package opencode implements agent.AgentProvider for the OpenCode CLI.
//
// Expected environment variables (set via Options.Env):
//   - OPENCODE_API_KEY — API key when required by the configured model/provider.
package opencode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
)

var _ agent.AgentProvider = (*Provider)(nil)

// Options configures an OpenCode provider instance.
type Options struct {
	Variant                    string
	Agent                      string
	DangerouslySkipPermissions bool
	Env                        map[string]string
}

// Provider launches opencode run --format json and normalizes its JSONL stdout.
type Provider struct {
	model string
	opts  Options
}

// NewProvider returns an OpenCode agent provider for the given model.
func NewProvider(model string, opts Options) *Provider {
	return &Provider{model: model, opts: opts}
}

func (p *Provider) Name() string { return "opencode" }

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

func (p *Provider) BuildCommand(prompt string) []string {
	args := []string{"opencode", "run", "--format", "json", "--model", p.model}
	if p.opts.Variant != "" {
		args = append(args, "--variant", p.opts.Variant)
	}
	if p.opts.Agent != "" {
		args = append(args, "--agent", p.opts.Agent)
	}
	if p.opts.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	return append(args, prompt)
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
	case "step_start":
		if raw.SessionID == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventSessionID, SessionID: raw.SessionID}}, nil

	case "text":
		if raw.Part == nil || raw.Part.Type != "text" || raw.Part.Text == "" {
			return nil, nil
		}
		text := raw.Part.Text
		return []agent.AgentEvent{
			{Kind: agent.EventResult, Result: &agent.Result{Output: text}},
		}, nil

	case "tool_use":
		return parseToolUse(raw.Part)

	case "error":
		msg := extractErrorMessage(raw.Error)
		if msg == "" {
			return nil, nil
		}
		return []agent.AgentEvent{{Kind: agent.EventResult, Result: &agent.Result{Output: msg}}}, nil

	case "step_finish":
		if raw.Part == nil || raw.Part.Tokens == nil {
			return nil, nil
		}
		if raw.Part.Tokens.Input == 0 && raw.Part.Tokens.Output == 0 {
			return nil, nil
		}
		return []agent.AgentEvent{{
			Kind: agent.EventUsage,
			Usage: &agent.Usage{
				InputTokens:  raw.Part.Tokens.Input,
				OutputTokens: raw.Part.Tokens.Output,
			},
		}}, nil

	default:
		return nil, nil
	}
}

func parseToolUse(part *streamPart) ([]agent.AgentEvent, error) {
	if part == nil || part.Type != "tool" || part.Tool == "" {
		return nil, nil
	}
	if part.State == nil || part.State.Status != "completed" {
		return nil, nil
	}
	args, ok := toolArgs(part.Tool, part.State.Input)
	if !ok {
		return nil, nil
	}
	return []agent.AgentEvent{{
		Kind:     agent.EventToolCall,
		ToolCall: &agent.ToolCall{Name: part.Tool, Args: args},
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
	case "webfetch":
		if v, ok := stringField(input, "url"); ok {
			return v, true
		}
	case "task":
		if v, ok := stringField(input, "description"); ok {
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

func extractErrorMessage(err *streamError) string {
	if err == nil {
		return ""
	}
	if err.Data != nil && err.Data.Message != "" {
		return err.Data.Message
	}
	if err.Message != "" {
		return err.Message
	}
	return err.Name
}

type streamEvent struct {
	Type      string       `json:"type"`
	SessionID string       `json:"sessionID"`
	Part      *streamPart  `json:"part"`
	Error     *streamError `json:"error"`
}

type streamPart struct {
	Type   string            `json:"type"`
	Text   string            `json:"text"`
	Tool   string            `json:"tool"`
	State  *streamToolState  `json:"state"`
	Tokens *streamTokenUsage `json:"tokens"`
}

type streamToolState struct {
	Status string                     `json:"status"`
	Input  map[string]json.RawMessage `json:"input"`
}

type streamTokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

type streamError struct {
	Name    string            `json:"name"`
	Message string            `json:"message"`
	Data    *streamErrorData  `json:"data"`
}

type streamErrorData struct {
	Message string `json:"message"`
}
