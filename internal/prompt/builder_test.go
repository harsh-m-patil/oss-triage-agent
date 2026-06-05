package prompt_test

import (
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
)

func TestBuilder_ForIssue_includesIssueContextAndCompletionSignal(t *testing.T) {
	t.Parallel()

	got := prompt.Builder{}.ForIssue(issue.Issue{
		Number: 9,
		Title:  "Wire build workflow",
		Body:   "Implement the end-to-end build command.",
	})

	for _, want := range []string{
		"Wire build workflow",
		"Implement the end-to-end build command.",
		orchestrator.CompletionSignal,
		"emit",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}
