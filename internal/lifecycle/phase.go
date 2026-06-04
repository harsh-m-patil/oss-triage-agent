package lifecycle

// Phase names a stage in an AFK run.
type Phase string

const (
	PhaseStart    Phase = "start"
	PhaseTriage   Phase = "triage"
	PhaseComplete Phase = "complete"
)
