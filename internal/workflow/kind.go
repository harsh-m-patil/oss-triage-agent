package workflow

// Kind identifies a supported AFK workflow.
type Kind string

const (
	KindTriage Kind = "triage"
	KindPlan   Kind = "plan"
	KindBuild  Kind = "build"
)
