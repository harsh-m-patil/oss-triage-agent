package fake

import (
	"context"
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

// Issue is a convenience alias for tests seeding fake trackers.
type Issue = issue.Issue

var _ issue.IssueTracker = (*Tracker)(nil)

// Tracker is a test double for issue.IssueTracker.
type Tracker struct {
	issues map[string]Issue
}

// NewTracker returns a fake issue tracker backed by the given issues.
func NewTracker(issues map[string]Issue) *Tracker {
	return &Tracker{issues: issues}
}

func (t *Tracker) ReadIssue(_ context.Context, id string) (*issue.Issue, error) {
	it, ok := t.issues[id]
	if !ok {
		return nil, fmt.Errorf("issue %q not found", id)
	}
	return &it, nil
}

func (t *Tracker) ListIssues(_ context.Context, _ string) ([]issue.Issue, error) {
	out := make([]issue.Issue, 0, len(t.issues))
	for _, it := range t.issues {
		out = append(out, it)
	}
	return out, nil
}

func (t *Tracker) Comment(_ context.Context, _ string, _ string) error { return nil }

func (t *Tracker) AddLabel(_ context.Context, id, label string) error {
	it, ok := t.issues[id]
	if !ok {
		return fmt.Errorf("issue %q not found", id)
	}
	it.Labels = append(it.Labels, label)
	t.issues[id] = it
	return nil
}

func (t *Tracker) RemoveLabel(_ context.Context, id, label string) error {
	it, ok := t.issues[id]
	if !ok {
		return fmt.Errorf("issue %q not found", id)
	}
	filtered := it.Labels[:0]
	for _, l := range it.Labels {
		if l != label {
			filtered = append(filtered, l)
		}
	}
	it.Labels = filtered
	t.issues[id] = it
	return nil
}
