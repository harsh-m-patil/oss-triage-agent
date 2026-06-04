package prompt

import "github.com/harsh-m-patil/oss-triage-agent/internal/issue"

// Builder renders agent prompts from issue context (stub).
type Builder struct{}

// ForIssue returns a minimal prompt for the given issue.
func (b Builder) ForIssue(it issue.Issue) string {
	return it.Title + "\n\n" + it.Body
}
