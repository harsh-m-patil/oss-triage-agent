package nosandbox

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
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
		recordStreamErr(streamLines(stdout, onStdout))
	}()
	go func() {
		defer wg.Done()
		recordStreamErr(streamLines(stderr, onStderr))
	}()

	wg.Wait()
	waitErr := cmd.Wait()
	if streamErr != nil {
		return errors.Join(streamErr, waitErr)
	}
	return waitErr
}

func streamLines(r io.Reader, onLine func(line string)) error {
	if onLine == nil {
		_, err := io.Copy(io.Discard, r)
		return err
	}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			onLine(line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
