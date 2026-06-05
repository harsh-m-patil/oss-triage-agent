package issue

import "context"

// Issue is a normalized work item from a tracker.
type Issue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// IssueTracker abstracts GitHub or other issue backends.
type IssueTracker interface {
	ReadIssue(ctx context.Context, id string) (*Issue, error)
	ListIssues(ctx context.Context, query string) ([]Issue, error)
	Comment(ctx context.Context, id string, body string) error
	AddLabel(ctx context.Context, id, label string) error
	RemoveLabel(ctx context.Context, id, label string) error
	Lock(ctx context.Context, id string) error
	Unlock(ctx context.Context, id string) error
}
