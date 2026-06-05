package config

// Git holds git worktree policy for AFK runs.
type Git struct {
	// RemoveCleanWorktreeOnSuccess removes an issue worktree after a successful
	// run when it has no uncommitted changes.
	RemoveCleanWorktreeOnSuccess bool `json:"remove_clean_worktree_on_success"`
}

// Config holds top-level runtime settings.
type Config struct {
	Workspace string `json:"workspace"`
	Git       Git    `json:"git"`
}
