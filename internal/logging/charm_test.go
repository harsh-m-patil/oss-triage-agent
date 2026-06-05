package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNewCharm_logsStructuredInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewCharm(&buf, "build")

	logger.Info(context.Background(), "reading issue", "issue_id", "9")

	got := buf.String()
	for _, want := range []string{"INFO", "reading issue", "issue_id=9"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q:\n%s", want, got)
		}
	}
}

func TestNewCharm_nilWriterDiscards(t *testing.T) {
	t.Parallel()

	logger := NewCharm(nil, "build")
	logger.Info(context.Background(), "discarded")
}

func TestCharmLogger_customLevels(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewCharm(&buf, "build")

	logger.Agent("session", "session_id", "sess_123")
	logger.Tool("bash", "args", "npm test")
	logger.Stderr("line", "line", "permission denied")

	got := buf.String()
	for _, want := range []string{"AGENT", "TOOL", "STDERR", "session_id=sess_123", "args=\"npm test\""} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q:\n%s", want, got)
		}
	}
}
