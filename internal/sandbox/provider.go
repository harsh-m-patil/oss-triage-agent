package sandbox

import "context"

// SandboxHandle is a running sandbox instance.
type SandboxHandle interface {
	Kind() SandboxKind
	Close() error
}

// SandboxProvider creates and tears down sandboxes for agent execution.
type SandboxProvider interface {
	Create(ctx context.Context, workspace string) (SandboxHandle, error)
}
