package git

import "context"

// Repository abstracts local git operations used by workflows.
type Repository interface {
	Clone(ctx context.Context, url, dir string) error
	WorktreePath() string
}
