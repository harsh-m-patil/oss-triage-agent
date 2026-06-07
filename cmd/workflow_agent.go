package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/pi"
)

const (
	workflowProviderOpenCode = "opencode"
	workflowProviderPi       = "pi"
)

// workflowAgentConfig holds agent provider flags shared by workflow CLIs.
type workflowAgentConfig struct {
	Provider                   string
	Model                      string
	Variant                    string
	AgentName                  string
	Thinking                   string
	Session                    string
	DangerouslySkipPermissions bool
}

func resolveWorkflowAgent(cfg workflowAgentConfig) (agent.AgentProvider, error) {
	switch strings.TrimSpace(cfg.Provider) {
	case "", workflowProviderOpenCode:
		return opencode.NewProvider(cfg.Model, opencode.Options{
			Variant:                    cfg.Variant,
			Agent:                      cfg.AgentName,
			DangerouslySkipPermissions: cfg.DangerouslySkipPermissions,
			Env:                        opencodeEnvFromOS(),
		}), nil
	case workflowProviderPi:
		return pi.NewProvider(cfg.Model, pi.Options{
			Thinking:      cfg.Thinking,
			ResumeSession: cfg.Session,
			Env:           piEnvFromOS(),
		}), nil
	default:
		return nil, fmt.Errorf("unsupported agent provider %q (want %s or %s)", cfg.Provider, workflowProviderOpenCode, workflowProviderPi)
	}
}

func workflowAgentBinary(provider string) string {
	if strings.TrimSpace(provider) == workflowProviderPi {
		return "pi"
	}
	return "opencode"
}

func opencodeEnvFromOS() map[string]string {
	if v := os.Getenv("OPENCODE_API_KEY"); v != "" {
		return map[string]string{"OPENCODE_API_KEY": v}
	}
	return nil
}

func piEnvFromOS() map[string]string {
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		return map[string]string{"ANTHROPIC_API_KEY": v}
	}
	return nil
}
