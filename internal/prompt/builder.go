package prompt

import (
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

// Builder renders agent prompts from issue context (stub).
type Builder struct{}

// ForIssue returns a build-oriented prompt for the given issue.
func (b Builder) ForIssue(it issue.Issue) string {
	return fmt.Sprintf(
		"You are running the AFK build workflow for issue #%d.\n\nTitle: %s\n\nIssue body:\n%s",
		it.Number,
		it.Title,
		it.Body,
	)
}
