package nosandbox

import (
	"context"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

var (
	_ sandbox.SandboxProvider = (*Provider)(nil)
	_ sandbox.SandboxHandle   = (*handle)(nil)
)

// Provider runs commands on the host workspace with no isolation.
type Provider struct{}

// NewProvider returns a host-execution sandbox provider.
func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Create(_ context.Context, workspace string) (sandbox.SandboxHandle, error) {
	return &handle{workspace: workspace}, nil
}

type handle struct {
	workspace string
}

func (h *handle) Kind() sandbox.SandboxKind { return sandbox.SandboxNone }

func (h *handle) WorkspacePath() string { return h.workspace }

func (h *handle) Exec(ctx context.Context, command string, args []string, stdin string, env map[string]string, onStdout, onStderr func(line string)) error {
	return runCommand(ctx, h.workspace, command, args, stdin, env, onStdout, onStderr)
}

func (h *handle) Close() error { return nil }
