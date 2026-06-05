package prompt_test

import (
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
)

func TestBuilder_ForTriage_includesIssueContextAndPolicy(t *testing.T) {
	t.Parallel()

	got := prompt.Builder{}.ForTriage(issue.Issue{
		Number: 12,
		Title:  "Triage workflow",
		Body:   "Implement automated triage.",
		Labels: []string{"needs-triage"},
	})

	for _, want := range []string{
		"Triage workflow",
		"Implement automated triage.",
		"needs-triage",
		"ready-for-agent",
		"```json",
		"# Triage skill (SKILL.md)",
		"# Agent brief guide (AGENT-BRIEF.md)",
		"# Out of scope guide (OUT-OF-SCOPE.md)",
		"Durability over precision",
		"Out-of-Scope Knowledge Base",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

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
