package agent

// EventKind identifies normalized agent stream events.
type EventKind string

const (
	EventText      EventKind = "text"
	EventResult    EventKind = "result"
	EventToolCall  EventKind = "tool_call"
	EventSessionID EventKind = "session_id"
	EventUsage     EventKind = "usage"
)

// AgentEvent is a single normalized event from an agent stream.
type AgentEvent struct {
	Kind      EventKind
	Text      string
	Result    *Result
	ToolCall  *ToolCall
	SessionID string
	Usage     *Usage
}

// Result captures a final agent outcome.
type Result struct {
	Output string
}

// ToolCall captures an agent tool invocation.
type ToolCall struct {
	Name string
	Args string
}

// Usage captures token or cost accounting from the agent.
type Usage struct {
	InputTokens  int
	OutputTokens int
}
