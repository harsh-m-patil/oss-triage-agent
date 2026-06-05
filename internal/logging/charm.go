package logging

import (
	"context"
	"io"

	charm "github.com/charmbracelet/log"
)

// CharmLogger implements Logger using charmbracelet/log.
type CharmLogger struct {
	l *charm.Logger
}

// NewCharm returns a workflow logger backed by charmbracelet/log.
// When w is nil, logs are discarded.
func NewCharm(w io.Writer, prefix string) *CharmLogger {
	if w == nil {
		w = io.Discard
	}
	l := charm.NewWithOptions(w, charm.Options{
		Prefix:          prefix,
		ReportTimestamp: false,
		ReportCaller:    false,
	})
	applyBuildStyles(l)
	return &CharmLogger{l: l}
}

// Charm returns the underlying charmbracelet logger for structured progress output.
func (c *CharmLogger) Charm() *charm.Logger {
	if c == nil || c.l == nil {
		return charm.New(io.Discard)
	}
	return c.l
}

func (c *CharmLogger) Info(ctx context.Context, msg string, keysAndValues ...any) {
	if c == nil || c.l == nil {
		return
	}
	c.l.Info(msg, keysAndValues...)
}

func (c *CharmLogger) Error(ctx context.Context, msg string, keysAndValues ...any) {
	if c == nil || c.l == nil {
		return
	}
	c.l.Error(msg, keysAndValues...)
}

func (c *CharmLogger) log(level charm.Level, msg string, keysAndValues ...any) {
	if c == nil || c.l == nil {
		return
	}
	c.l.Log(level, msg, keysAndValues...)
}

// Agent logs normalized agent stream output.
func (c *CharmLogger) Agent(msg string, keysAndValues ...any) {
	c.log(LevelAgent, msg, keysAndValues...)
}

// Tool logs agent tool invocations.
func (c *CharmLogger) Tool(msg string, keysAndValues ...any) {
	c.log(LevelTool, msg, keysAndValues...)
}

// Usage logs agent token accounting.
func (c *CharmLogger) Usage(msg string, keysAndValues ...any) {
	c.log(LevelUsage, msg, keysAndValues...)
}

// Stderr logs agent stderr lines.
func (c *CharmLogger) Stderr(msg string, keysAndValues ...any) {
	c.log(LevelStderr, msg, keysAndValues...)
}

// Heartbeat logs idle-wait progress while the agent is running.
func (c *CharmLogger) Heartbeat(msg string, keysAndValues ...any) {
	c.log(LevelHeartbeat, msg, keysAndValues...)
}
