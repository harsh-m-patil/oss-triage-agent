package agent_test

import (
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/fake"
)

func TestFakeProvider_ParseStreamLine_returnsTextEvent(t *testing.T) {
	t.Parallel()

	p := fake.NewProvider()
	events, err := p.ParseStreamLine(`{"type":"text","content":"hello"}`)
	if err != nil {
		t.Fatalf("ParseStreamLine: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Kind != agent.EventText {
		t.Fatalf("Kind = %q, want %q", events[0].Kind, agent.EventText)
	}
	if events[0].Text != "hello" {
		t.Fatalf("Text = %q, want hello", events[0].Text)
	}
}
