package sandbox

// SandboxKind describes how a sandbox relates to the host workspace.
type SandboxKind string

const (
	SandboxBindMount SandboxKind = "bind-mount"
	SandboxIsolated  SandboxKind = "isolated"
	SandboxNone      SandboxKind = "none"
)
