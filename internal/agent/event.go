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
	Kind      EventKind  `json:"kind"`
	Text      string     `json:"text,omitempty"`
	Result    *Result    `json:"result,omitempty"`
	ToolCall  *ToolCall  `json:"tool_call,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"`
}

// Result captures a final agent outcome.
type Result struct {
	Output string `json:"output"`
}

// ToolCall captures an agent tool invocation.
type ToolCall struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// Usage captures token or cost accounting from the agent.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
