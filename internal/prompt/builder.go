package prompt

import (
	"fmt"
	"strings"

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

// ForTriage returns a triage-oriented prompt for the given issue.
func (b Builder) ForTriage(it issue.Issue) string {
	labels := "none"
	if len(it.Labels) > 0 {
		labels = strings.Join(it.Labels, ", ")
	}
	return fmt.Sprintf(`You are running the AFK triage workflow for issue #%d.

Title: %s
Current labels: %s

Issue body:
%s

This is a fully automated run. Follow the triage skill and reference docs below.
Do not wait for maintainer input — assess the issue, choose category/state, and
produce the issue comment yourself.

---

# Triage skill (SKILL.md)

%s

---

# Agent brief guide (AGENT-BRIEF.md)

%s

---

# Out of scope guide (OUT-OF-SCOPE.md)

%s

---

## AFK automation

Label contract (from CONTEXT.md):
- Every triaged issue gets exactly one category label (bug or enhancement) and one state label.
- Do not use agent:in-progress during triage.

Your task:
1. Gather context: read the issue, explore the repository, and check .out-of-scope/*.md for prior rejections.
2. For bugs, attempt reproduction before deciding state.
3. Write the issue comment:
   - ready-for-agent — agent brief per AGENT-BRIEF.md
   - ready-for-human — same structure, note why a human must implement
   - needs-info — use the needs-info template from SKILL.md
   - wontfix — explain why; for enhancements, note the matching .out-of-scope/ concept if one applies
4. End your response with a fenced JSON block (parsed by automation):

`+"```json\n"+`{"category":"<bug|enhancement>","state":"<state>","close":<true|false>}
`+"```"+`

Set close to true only for wontfix. Do not include the AI disclaimer in your output; the CLI adds it.`,
		it.Number,
		it.Title,
		labels,
		it.Body,
		triageSkillDoc,
		agentBriefDoc,
		outOfScopeDoc,
	)
}
