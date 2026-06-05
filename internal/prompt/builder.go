package prompt

import (
	"fmt"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
)

// Builder renders agent prompts from issue context (stub).
type Builder struct{}

// ForIssue returns a build-oriented prompt for the given issue.
func (b Builder) ForIssue(it issue.Issue) string {
	return fmt.Sprintf(
		"You are running the AFK build workflow for issue #%d.\n\nTitle: %s\n\nIssue body:\n%s\n\nWhen the work is complete, emit %s exactly once on stdout.",
		it.Number,
		it.Title,
		it.Body,
		orchestrator.CompletionSignal,
	)
}
