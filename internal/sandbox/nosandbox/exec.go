package nosandbox

import (
	"bufio"
	"context"
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
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamLines(stdout, onStdout)
	}()
	go func() {
		defer wg.Done()
		streamLines(stderr, onStderr)
	}()

	wg.Wait()
	return cmd.Wait()
}

func streamLines(r io.Reader, onLine func(line string)) {
	if onLine == nil {
		io.Copy(io.Discard, r)
		return
	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		onLine(sc.Text())
	}
}
