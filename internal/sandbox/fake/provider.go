package fake

import (
	"context"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

var _ sandbox.SandboxProvider = (*Provider)(nil)

// Provider is a test double for sandbox.SandboxProvider.
type Provider struct {
	kind sandbox.SandboxKind
}

// NewProvider returns a fake sandbox provider that reports the given kind.
func NewProvider(kind sandbox.SandboxKind) *Provider {
	return &Provider{kind: kind}
}

func (p *Provider) Create(_ context.Context, _ string) (sandbox.SandboxHandle, error) {
	return &handle{kind: p.kind}, nil
}

type handle struct {
	kind sandbox.SandboxKind
}

func (h *handle) Kind() sandbox.SandboxKind { return h.kind }

func (h *handle) Close() error { return nil }
