package prompt

import _ "embed"

//go:embed triage/SKILL.md
var triageSkillDoc string

//go:embed triage/AGENT-BRIEF.md
var agentBriefDoc string

//go:embed triage/OUT-OF-SCOPE.md
var outOfScopeDoc string
