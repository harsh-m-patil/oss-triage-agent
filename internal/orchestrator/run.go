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

// CompletionSignal is the authoritative AFK success token in agent stdout.
const CompletionSignal = "<promise>COMPLETE</promise>"

// TimeoutKind identifies which orchestrator timeout ended a run.
type TimeoutKind string

const (
	// TimeoutIdle means no stdout arrived within IdleTimeout before the completion signal.
	TimeoutIdle TimeoutKind = "idle"
)

// ErrIdleTimeout is returned when IdleTimeout expires before CompletionSignal.
var ErrIdleTimeout = errors.New("idle timeout waiting for completion signal")

var progressHeartbeatInterval = 30 * time.Second

func (o *Orchestrator) runAgent(
	ctx context.Context,
	handle sandbox.SandboxHandle,
	command string,
	args []string,
	env map[string]string,
	idleTimeout, completionTimeout time.Duration,
	progress func(ProgressEvent),
	summary *RunSummary,
) error {
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var (
		sawComplete bool
		events      []agent.AgentEvent
		parseErr    error
		stderrTail  []string
		lastStdout  = time.Now()
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
	stopIdle := func() {}
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
		stopIdle = func() {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
		}
		defer idleTimer.Stop()
	}

	var heartbeatC <-chan time.Time
	if progress != nil && progressHeartbeatInterval > 0 {
		ticker := time.NewTicker(progressHeartbeatInterval)
		heartbeatC = ticker.C
		defer ticker.Stop()
	}

	completionDone := make(chan struct{}, 1)
	var completionTimer *time.Timer
	startCompletionTimer := func() {
		if completionTimeout <= 0 || completionTimer != nil {
			return
		}
		completionTimer = time.AfterFunc(completionTimeout, func() {
			select {
			case completionDone <- struct{}{}:
			default:
			}
		})
	}

	onStdout := func(line string) {
		var (
			emitEvents      []agent.AgentEvent
			emitCompletion  bool
		)
		mu.Lock()
		if parseErr != nil {
			mu.Unlock()
			return
		}

		lastStdout = time.Now()
		resetIdle()

		if strings.Contains(line, CompletionSignal) {
			sawComplete = true
			stopIdle()
			startCompletionTimer()
			emitCompletion = true
		}

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

		if emitCompletion {
			emit(ProgressEvent{
				Kind:              ProgressCompletionSignal,
				CompletionTimeout: completionTimeout,
			})
		}
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
		execDone <- handle.Exec(execCtx, command, args, env, onStdout, onStderr)
	}()

	for {
		select {
		case err := <-execDone:
			mu.Lock()
			defer mu.Unlock()
			summary.Events = events
			summary.Completed = sawComplete
			if parseErr != nil {
				return parseErr
			}
			if sawComplete {
				summary.Success = true
				return nil
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				return formatAgentExecError(o.deps.Agent.Name(), command, err, stderrTail)
			}
			return fmt.Errorf("process exited without %s", CompletionSignal)

		case <-idleC:
			mu.Lock()
			complete := sawComplete
			mu.Unlock()
			if complete {
				continue
			}
			cancel()
			<-execDone
			mu.Lock()
			summary.Events = events
			summary.Completed = false
			summary.TimeoutKind = TimeoutIdle
			mu.Unlock()
			return ErrIdleTimeout

		case <-completionDone:
			cancel()
			mu.Lock()
			summary.Events = events
			summary.Completed = sawComplete
			summary.Success = sawComplete
			mu.Unlock()
			return nil

		case <-heartbeatC:
			mu.Lock()
			complete := sawComplete
			wait := time.Since(lastStdout)
			mu.Unlock()
			emit(ProgressEvent{
				Kind:      ProgressHeartbeat,
				Completed: complete,
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
