package issue

import "context"

// Issue is a normalized work item from a tracker.
type Issue struct {
	Number int
	Title  string
	Body   string
	Labels []string
}

// IssueTracker abstracts GitHub or other issue backends.
type IssueTracker interface {
	ReadIssue(ctx context.Context, id string) (*Issue, error)
	ListIssues(ctx context.Context, query string) ([]Issue, error)
	Comment(ctx context.Context, id string, body string) error
	AddLabel(ctx context.Context, id, label string) error
	RemoveLabel(ctx context.Context, id, label string) error
}
