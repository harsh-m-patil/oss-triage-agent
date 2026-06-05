package sandbox_test

import (
	"context"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/fake"
)

func TestFakeProvider_Create_returnsBindMountHandle(t *testing.T) {
	t.Parallel()

	p := fake.NewProvider(sandbox.SandboxBindMount)
	handle, err := p.Create(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	if handle.Kind() != sandbox.SandboxBindMount {
		t.Fatalf("Kind = %q, want %q", handle.Kind(), sandbox.SandboxBindMount)
	}
	if handle.WorkspacePath() != "/workspace" {
		t.Fatalf("WorkspacePath = %q, want /workspace", handle.WorkspacePath())
	}
}
