package agent

// AgentProvider abstracts how an AFK coding agent is launched and how its
// stdout stream lines become structured events.
type AgentProvider interface {
	Name() string
	Env() map[string]string
	BuildCommand(prompt string) []string
	ParseStreamLine(line string) ([]AgentEvent, error)
}
