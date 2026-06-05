package sandbox

import "context"

// SandboxHandle is a running sandbox instance.
type SandboxHandle interface {
	Kind() SandboxKind
	// WorkspacePath is the directory where commands run (agent cwd).
	WorkspacePath() string
	// Exec runs a subprocess in the workspace. onStdout and onStderr are
	// invoked once per output line as data arrives; a final partial line
	// without a trailing newline is flushed when the process exits.
	// Returns the process exit error (nil on success).
	Exec(ctx context.Context, command string, args []string, onStdout, onStderr func(line string)) error
	Close() error
}

// SandboxProvider creates and tears down sandboxes for agent execution.
type SandboxProvider interface {
	Create(ctx context.Context, workspace string) (SandboxHandle, error)
}
