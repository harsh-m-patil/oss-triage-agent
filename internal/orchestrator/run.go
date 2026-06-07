package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

// TimeoutKind identifies which orchestrator timeout ended a run.
type TimeoutKind string

const (
	// TimeoutIdle means no stdout arrived within IdleTimeout before the agent exited.
	TimeoutIdle TimeoutKind = "idle"
)

// ErrIdleTimeout is returned when IdleTimeout expires before the agent exits.
var ErrIdleTimeout = errors.New("idle timeout waiting for agent exit")

var progressHeartbeatInterval = 30 * time.Second

func (o *Orchestrator) runAgent(
	ctx context.Context,
	handle sandbox.SandboxHandle,
	command string,
	args []string,
	stdin string,
	env map[string]string,
	idleTimeout time.Duration,
	progress func(ProgressEvent),
	summary *RunSummary,
) error {
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var (
		events     []agent.AgentEvent
		parseErr   error
		stderrTail []string
		lastStdout = time.Now()
	)

	emit := func(ev ProgressEvent) {
		if progress == nil {
			return
		}
		progress(ev)
	}
	emit(ProgressEvent{
		Kind:    ProgressAgentStart,
		Command: command,
		Args:    append([]string(nil), args...),
	})

	resetIdle := func() {}
	var idleC <-chan time.Time
	if idleTimeout > 0 {
		idleTimer := time.NewTimer(idleTimeout)
		idleC = idleTimer.C
		resetIdle = func() {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idleTimeout)
		}
		defer idleTimer.Stop()
	}

	var heartbeatC <-chan time.Time
	if progress != nil && progressHeartbeatInterval > 0 {
		ticker := time.NewTicker(progressHeartbeatInterval)
		heartbeatC = ticker.C
		defer ticker.Stop()
	}

	onStdout := func(line string) {
		var emitEvents []agent.AgentEvent
		mu.Lock()
		if parseErr != nil {
			mu.Unlock()
			return
		}

		lastStdout = time.Now()
		resetIdle()

		evts, err := o.deps.Agent.ParseStreamLine(line)
		if err != nil {
			parseErr = err
			mu.Unlock()
			cancel()
			return
		}
		events = append(events, evts...)
		emitEvents = append(emitEvents, evts...)
		mu.Unlock()
		for i := range emitEvents {
			ev := emitEvents[i]
			emit(ProgressEvent{
				Kind:  ProgressAgentEvent,
				Event: &ev,
			})
		}
	}

	onStderr := func(line string) {
		mu.Lock()
		stderrTail = appendTailLine(stderrTail, line, 20)
		mu.Unlock()
		emit(ProgressEvent{
			Kind:       ProgressAgentStderr,
			StderrLine: line,
		})
	}

	execDone := make(chan error, 1)
	go func() {
		execDone <- handle.Exec(execCtx, command, args, stdin, env, onStdout, onStderr)
	}()

	for {
		select {
		case err := <-execDone:
			mu.Lock()
			defer mu.Unlock()
			summary.Events = events
			if parseErr != nil {
				return parseErr
			}
			if err == nil {
				summary.Completed = true
				summary.Success = true
				return nil
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				return formatAgentExecError(o.deps.Agent.Name(), command, err, stderrTail)
			}
			return fmt.Errorf("process exited before agent completed")

		case <-idleC:
			cancel()
			<-execDone
			mu.Lock()
			summary.Events = events
			summary.Completed = false
			summary.TimeoutKind = TimeoutIdle
			mu.Unlock()
			return ErrIdleTimeout

		case <-heartbeatC:
			mu.Lock()
			wait := time.Since(lastStdout)
			mu.Unlock()
			emit(ProgressEvent{
				Kind:      ProgressHeartbeat,
				Completed: false,
				Wait:      wait,
			})
		}
	}
}

func appendTailLine(lines []string, line string, max int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return lines
	}
	lines = append(lines, line)
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	return lines
}

func formatAgentExecError(agentName, command string, err error, stderrTail []string) error {
	if len(stderrTail) == 0 {
		return fmt.Errorf("agent exec %s (%s): %w", agentName, command, err)
	}
	return fmt.Errorf("agent exec %s (%s): %w; stderr tail: %s", agentName, command, err, strings.Join(stderrTail, " | "))
}
