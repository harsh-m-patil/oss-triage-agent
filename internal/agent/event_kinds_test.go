package agent_test

import (
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/fake"
)

func TestFakeProvider_ParseStreamLine_returnsSessionIDEvent(t *testing.T) {
	t.Parallel()

	p := fake.NewProvider()
	events, err := p.ParseStreamLine(`{"type":"session_id","content":"sess-1"}`)
	if err != nil {
		t.Fatalf("ParseStreamLine: %v", err)
	}
	if len(events) != 1 || events[0].Kind != agent.EventSessionID {
		t.Fatalf("events = %+v, want session_id kind", events)
	}
	if events[0].SessionID != "sess-1" {
		t.Fatalf("SessionID = %q, want sess-1", events[0].SessionID)
	}
}
