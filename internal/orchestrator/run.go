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

func (o *Orchestrator) runAgent(
	ctx context.Context,
	handle sandbox.SandboxHandle,
	command string,
	args []string,
	env map[string]string,
	idleTimeout, completionTimeout time.Duration,
	summary *RunSummary,
) error {
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var (
		sawComplete bool
		events      []agent.AgentEvent
		parseErr    error
	)

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
		mu.Lock()
		defer mu.Unlock()
		if parseErr != nil {
			return
		}

		resetIdle()

		if strings.Contains(line, CompletionSignal) {
			sawComplete = true
			stopIdle()
			startCompletionTimer()
		}

		evts, err := o.deps.Agent.ParseStreamLine(line)
		if err != nil {
			parseErr = err
			cancel()
			return
		}
		events = append(events, evts...)
	}

	execDone := make(chan error, 1)
	go func() {
		execDone <- handle.Exec(execCtx, command, args, env, onStdout, nil)
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
				return fmt.Errorf("agent exec: %w", err)
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
		}
	}
}
