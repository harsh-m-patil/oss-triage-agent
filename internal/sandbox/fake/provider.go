package fake

import (
	"context"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

var (
	_ sandbox.SandboxProvider = (*Provider)(nil)
	_ sandbox.SandboxHandle   = (*handle)(nil)
)

// Provider is a test double for sandbox.SandboxProvider.
type Provider struct {
	kind sandbox.SandboxKind
}

// NewProvider returns a fake sandbox provider that reports the given kind.
func NewProvider(kind sandbox.SandboxKind) *Provider {
	return &Provider{kind: kind}
}

func (p *Provider) Create(_ context.Context, workspace string) (sandbox.SandboxHandle, error) {
	return &handle{kind: p.kind, workspace: workspace}, nil
}

type handle struct {
	kind      sandbox.SandboxKind
	workspace string
}

func (h *handle) Kind() sandbox.SandboxKind { return h.kind }

func (h *handle) WorkspacePath() string { return h.workspace }

func (h *handle) Exec(_ context.Context, _ string, _ []string, _, _ func(line string)) error {
	return nil
}

func (h *handle) Close() error { return nil }
