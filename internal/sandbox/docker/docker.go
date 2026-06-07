package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
)

// WorkspaceInContainer is the bind-mounted directory where commands run inside the container.
const WorkspaceInContainer = "/workspace"

const (
	defaultBaseImage = "debian:bookworm-slim"
	defaultImageRepo = "oss-triage-agent/opencode"
	defaultImageTag  = "bookworm-slim-v3"
)

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
	imageRef, err := defaultImageRef()
	if err != nil {
		return nil, err
	}
	if err := p.ensureImage(ctx, imageRef); err != nil {
		return nil, err
	}

	binds := []string{fmt.Sprintf("%s:%s", workspace, WorkspaceInContainer)}

	resp, err := p.cli.ContainerCreate(ctx, &container.Config{
		Image: imageRef,
		Cmd:   []string{"sleep", "infinity"},
	}, &container.HostConfig{
		Binds: binds,
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
	buildContext, err := sandboxImageBuildContext()
	if err != nil {
		return err
	}
	resp, err := p.cli.ImageBuild(ctx, bytes.NewReader(buildContext), types.ImageBuildOptions{
		Tags:       []string{ref},
		Dockerfile: "Dockerfile",
		Remove:     true,
		PullParent: true,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := consumeImageBuildOutput(resp.Body); err != nil {
		return err
	}
	_, _, err = p.cli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return fmt.Errorf("inspect built image %q: %w", ref, err)
	}
	return nil
}

func defaultImageRef() (string, error) {
	arch, err := opencodeBinaryArch(runtime.GOARCH)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s-%s", defaultImageRepo, defaultImageTag, arch), nil
}

func sandboxImageBuildContext() ([]byte, error) {
	dockerfile := fmt.Sprintf(`
FROM %s

RUN apt-get update \
 && apt-get install -y --no-install-recommends bash ca-certificates curl git tar gnupg \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path \
 && ln -sf /root/.opencode/bin/opencode /usr/local/bin/opencode

RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && npm install -g --ignore-scripts @mariozechner/pi-coding-agent \
 && rm -rf /var/lib/apt/lists/*

WORKDIR %s
`, defaultBaseImage, WorkspaceInContainer)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Mode: 0o644,
		Size: int64(len(dockerfile)),
	}); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(dockerfile)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func opencodeBinaryArch(goArch string) (string, error) {
	switch goArch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported docker opencode architecture %q", goArch)
	}
}

type imageBuildEvent struct {
	Stream string `json:"stream"`
	Error  string `json:"error"`
}

func consumeImageBuildOutput(r io.Reader) error {
	dec := json.NewDecoder(r)
	var tail []string
	for {
		var event imageBuildEvent
		if err := dec.Decode(&event); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if event.Stream != "" {
			for _, line := range strings.Split(strings.TrimRight(event.Stream, "\n"), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				tail = append(tail, line)
				if len(tail) > 20 {
					tail = tail[len(tail)-20:]
				}
			}
		}
		if event.Error != "" {
			if len(tail) == 0 {
				return fmt.Errorf("docker image build failed: %s", event.Error)
			}
			return fmt.Errorf("docker image build failed: %s (tail: %s)", event.Error, strings.Join(tail, " | "))
		}
	}
}

type handle struct {
	cli         *client.Client
	containerID string

	closeOnce sync.Once
	closeErr  error
}

func (h *handle) Kind() sandbox.SandboxKind { return sandbox.SandboxBindMount }

func (h *handle) WorkspacePath() string { return WorkspaceInContainer }

func (h *handle) Exec(ctx context.Context, command string, args []string, stdin string, env map[string]string, onStdout, onStderr func(line string)) error {
	return runContainerExec(ctx, h.cli, h.containerID, WorkspaceInContainer, command, args, stdin, env, onStdout, onStderr)
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
