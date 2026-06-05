package nosandbox

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/streamio"
)

func runCommand(ctx context.Context, dir, command string, args []string, onStdout, onStderr func(line string)) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var streamErr error
	recordStreamErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if streamErr == nil {
			streamErr = err
		}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		recordStreamErr(streamio.Lines(stdout, onStdout))
	}()
	go func() {
		defer wg.Done()
		recordStreamErr(streamio.Lines(stderr, onStderr))
	}()

	wg.Wait()
	waitErr := cmd.Wait()
	if streamErr != nil {
		return errors.Join(streamErr, waitErr)
	}
	return waitErr
}
