package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

// WorkspaceInContainer is the bind-mounted directory where commands run inside the container.
const WorkspaceInContainer = "/workspace"

const defaultImage = "alpine:3.20"

var (
	_ sandbox.SandboxProvider = (*Provider)(nil)
	_ sandbox.SandboxHandle   = (*handle)(nil)
)

// Provider creates Docker sandboxes with the host workspace bind-mounted.
type Provider struct {
	cli *client.Client
}

// NewProvider returns a Docker sandbox provider using the environment's Docker socket.
func NewProvider() (*Provider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Provider{cli: cli}, nil
}

// Ping reports whether the Docker daemon is reachable.
func (p *Provider) Ping(ctx context.Context) error {
	_, err := p.cli.Ping(ctx)
	return err
}

func (p *Provider) Create(ctx context.Context, workspace string) (sandbox.SandboxHandle, error) {
	if err := ensureHostWorkspace(workspace); err != nil {
		return nil, err
	}
	if err := p.ensureImage(ctx, defaultImage); err != nil {
		return nil, err
	}

	resp, err := p.cli.ContainerCreate(ctx, &container.Config{
		Image: defaultImage,
		Cmd:   []string{"sleep", "infinity"},
	}, &container.HostConfig{
		Binds: []string{fmt.Sprintf("%s:%s", workspace, WorkspaceInContainer)},
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = p.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, err
	}

	return &handle{
		cli:         p.cli,
		containerID: resp.ID,
	}, nil
}

func ensureHostWorkspace(workspace string) error {
	info, err := os.Stat(workspace)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace %q is not a directory", workspace)
	}
	return nil
}

func (p *Provider) ensureImage(ctx context.Context, ref string) error {
	_, _, err := p.cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		return nil
	}
	rc, err := p.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = ioCopyDiscard(rc)
	return err
}

type handle struct {
	cli         *client.Client
	containerID string

	closeOnce sync.Once
	closeErr  error
}

func (h *handle) Kind() sandbox.SandboxKind { return sandbox.SandboxBindMount }

func (h *handle) WorkspacePath() string { return WorkspaceInContainer }

func (h *handle) Exec(ctx context.Context, command string, args []string, _ map[string]string, onStdout, onStderr func(line string)) error {
	return runContainerExec(ctx, h.cli, h.containerID, WorkspaceInContainer, command, args, onStdout, onStderr)
}

func (h *handle) Close() error {
	h.closeOnce.Do(func() {
		ctx := context.Background()
		timeout := 10
		_ = h.cli.ContainerStop(ctx, h.containerID, container.StopOptions{Timeout: &timeout})
		h.closeErr = h.cli.ContainerRemove(ctx, h.containerID, container.RemoveOptions{Force: true})
	})
	return h.closeErr
}

func ioCopyDiscard(r io.Reader) (int64, error) {
	return io.Copy(io.Discard, r)
}
