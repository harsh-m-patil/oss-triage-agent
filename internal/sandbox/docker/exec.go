package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/streamio"
)

func runContainerExec(
	ctx context.Context,
	cli *client.Client,
	containerID, workDir, command string,
	args []string,
	onStdout, onStderr func(line string),
) error {
	execCfg := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string{command}, args...),
		WorkingDir:   workDir,
	}
	execResp, err := cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return err
	}

	attach, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return err
	}
	defer attach.Close()

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	var copyErr error
	var copyWg sync.WaitGroup
	copyWg.Add(1)
	go func() {
		defer copyWg.Done()
		_, err := stdcopy.StdCopy(stdoutW, stderrW, attach.Reader)
		stdoutW.Close()
		stderrW.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			copyErr = err
		}
	}()

	var streamWg sync.WaitGroup
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

	streamWg.Add(2)
	go func() {
		defer streamWg.Done()
		recordStreamErr(streamio.Lines(stdoutR, onStdout))
	}()
	go func() {
		defer streamWg.Done()
		recordStreamErr(streamio.Lines(stderrR, onStderr))
	}()

	copyWg.Wait()
	streamWg.Wait()

	inspect, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return errors.Join(copyErr, streamErr, err)
	}

	var exitErr error
	if inspect.ExitCode != 0 {
		exitErr = fmt.Errorf("exec exited with code %d", inspect.ExitCode)
	}
	return errors.Join(copyErr, streamErr, exitErr)
}
