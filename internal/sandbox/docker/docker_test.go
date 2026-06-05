package docker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	dockerprovider "github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/docker"
)

func TestProvider_Create_returnsBindMountHandleWithContainerWorkspacePath(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("Docker daemon not available")
	}
	t.Parallel()

	dir := t.TempDir()
	hostWorkspace, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	p, err := dockerprovider.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	handle, err := p.Create(context.Background(), hostWorkspace)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	if handle.Kind() != sandbox.SandboxBindMount {
		t.Fatalf("Kind = %q, want %q", handle.Kind(), sandbox.SandboxBindMount)
	}
	if handle.WorkspacePath() != dockerprovider.WorkspaceInContainer {
		t.Fatalf("WorkspacePath = %q, want %q", handle.WorkspacePath(), dockerprovider.WorkspaceInContainer)
	}
	if _, err := os.Stat(hostWorkspace); err != nil {
		t.Fatalf("Stat host workspace: %v", err)
	}
}

func TestHandle_Exec_runsCommandInBindMountedWorkspace(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("Docker daemon not available")
	}
	t.Parallel()

	hostWorkspace := t.TempDir()
	marker := filepath.Join(hostWorkspace, "marker")
	if err := os.WriteFile(marker, []byte("mounted"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p, err := dockerprovider.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	handle, err := p.Create(context.Background(), hostWorkspace)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	var stdout []string
	err = handle.Exec(
		context.Background(),
		"cat",
		[]string{"marker"},
		nil,
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec cat: %v", err)
	}
	if len(stdout) != 1 || stdout[0] != "mounted" {
		t.Fatalf("stdout = %v, want [mounted]", stdout)
	}

	stdout = nil
	err = handle.Exec(
		context.Background(),
		"echo",
		[]string{"hello"},
		nil,
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec echo: %v", err)
	}
	if len(stdout) != 1 || stdout[0] != "hello" {
		t.Fatalf("stdout = %v, want [hello]", stdout)
	}
}

func TestHandle_Close_stopsContainerAndIsIdempotent(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("Docker daemon not available")
	}
	t.Parallel()

	p, err := dockerprovider.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := handle.Exec(context.Background(), "echo", []string{"before"}, nil, nil, nil); err != nil {
		t.Fatalf("Exec before Close: %v", err)
	}

	if err := handle.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	err = handle.Exec(context.Background(), "echo", []string{"after"}, nil, nil, nil)
	if err == nil {
		t.Fatal("Exec after Close: want error, got nil")
	}
}

func TestHandle_Exec_invokesStdoutCallbackWhileProcessRuns(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("Docker daemon not available")
	}
	t.Parallel()

	p, err := dockerprovider.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	firstLine := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- handle.Exec(
			context.Background(),
			"sh",
			[]string{"-c", "echo first; sleep 0.2; echo second"},
			nil,
			func(line string) {
				if line == "first" {
					close(firstLine)
				}
			},
			nil,
		)
	}()

	select {
	case <-firstLine:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first stdout line")
	}

	select {
	case err := <-done:
		t.Fatalf("Exec finished before second line: %v", err)
	default:
	}

	if err := <-done; err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestHandle_Exec_returnsWhenContextCancelled(t *testing.T) {
	if !dockerAvailable(t) {
		t.Skip("Docker daemon not available")
	}
	t.Parallel()

	p, err := dockerprovider.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- handle.Exec(ctx, "sleep", []string{"3600"}, nil, nil, nil)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Exec error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Exec did not return after context deadline")
	}
}

func dockerAvailable(t *testing.T) bool {
	t.Helper()
	p, err := dockerprovider.NewProvider()
	if err != nil {
		return false
	}
	return p.Ping(context.Background()) == nil
}
