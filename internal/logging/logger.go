package logging

import "context"

// Logger is the minimal logging contract used by workflows.
type Logger interface {
	Info(ctx context.Context, msg string, keysAndValues ...any)
	Error(ctx context.Context, msg string, keysAndValues ...any)
}
