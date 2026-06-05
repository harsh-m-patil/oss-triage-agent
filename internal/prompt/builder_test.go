package prompt_test

import (
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
)

func TestBuilder_ForIssue_includesIssueContext(t *testing.T) {
	t.Parallel()

	got := prompt.Builder{}.ForIssue(issue.Issue{
		Number: 9,
		Title:  "Wire build workflow",
		Body:   "Implement the end-to-end build command.",
	})

	for _, want := range []string{
		"Wire build workflow",
		"Implement the end-to-end build command.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}
