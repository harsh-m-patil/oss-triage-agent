package fake

import (
	"encoding/json"
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
)

// Provider is a test double for agent.AgentProvider.
type Provider struct{}

var _ agent.AgentProvider = (*Provider)(nil)

// NewProvider returns a fake agent provider for contract tests.
func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string { return "fake" }

func (p *Provider) Env() map[string]string { return nil }

func (p *Provider) BuildLaunch(prompt string) agent.Launch {
	return agent.Launch{Argv: []string{"echo", prompt}}
}

func (p *Provider) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	var raw struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse stream line: %w", err)
	}
	switch raw.Type {
	case "text":
		return []agent.AgentEvent{{Kind: agent.EventText, Text: raw.Content}}, nil
	case "session_id":
		return []agent.AgentEvent{{Kind: agent.EventSessionID, SessionID: raw.Content}}, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", raw.Type)
	}
}
