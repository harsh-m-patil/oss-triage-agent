package config

// Git holds git worktree policy for AFK runs.
type Git struct {
	// RemoveCleanWorktreeOnSuccess removes an issue worktree after a successful
	// run when it has no uncommitted changes.
	RemoveCleanWorktreeOnSuccess bool
}

// Config holds top-level runtime settings.
type Config struct {
	Workspace string
	Git       Git
}
