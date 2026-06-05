package nosandbox_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/nosandbox"
)

func TestProvider_Create_returnsNoneHandleBoundToWorkspace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workspace, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	p := nosandbox.NewProvider()
	handle, err := p.Create(context.Background(), workspace)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	if handle.Kind() != sandbox.SandboxNone {
		t.Fatalf("Kind = %q, want %q", handle.Kind(), sandbox.SandboxNone)
	}
	path := handle.WorkspacePath()
	if path != workspace {
		t.Fatalf("WorkspacePath = %q, want %q", path, workspace)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat workspace: %v", err)
	}
}

func TestHandle_Exec_streamsStdoutLinesInOrder(t *testing.T) {
	t.Parallel()

	p := nosandbox.NewProvider()
	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	var stdout []string
	err = handle.Exec(
		context.Background(),
		"sh",
		[]string{"-c", "echo one; echo two; echo three"},
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	want := []string{"one", "two", "three"}
	if len(stdout) != len(want) {
		t.Fatalf("stdout lines = %v, want %v", stdout, want)
	}
	for i := range want {
		if stdout[i] != want[i] {
			t.Fatalf("stdout[%d] = %q, want %q (full: %v)", i, stdout[i], want[i], stdout)
		}
	}
}

func TestHandle_Exec_deliversEmptyStdoutLines(t *testing.T) {
	t.Parallel()

	p := nosandbox.NewProvider()
	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	var stdout []string
	err = handle.Exec(
		context.Background(),
		"sh",
		[]string{"-c", "printf 'a\n\nb\n'"},
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	want := []string{"a", "", "b"}
	if len(stdout) != len(want) {
		t.Fatalf("stdout lines = %v, want %v", stdout, want)
	}
	for i := range want {
		if stdout[i] != want[i] {
			t.Fatalf("stdout[%d] = %q, want %q", i, stdout[i], want[i])
		}
	}
}

func TestHandle_Exec_flushesPartialStdoutLineOnExit(t *testing.T) {
	t.Parallel()

	p := nosandbox.NewProvider()
	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	var stdout []string
	err = handle.Exec(
		context.Background(),
		"sh",
		[]string{"-c", "printf 'no trailing newline'"},
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if len(stdout) != 1 || stdout[0] != "no trailing newline" {
		t.Fatalf("stdout = %v, want [no trailing newline]", stdout)
	}
}

func TestHandle_Exec_invokesStdoutCallbackWhileProcessRuns(t *testing.T) {
	t.Parallel()

	p := nosandbox.NewProvider()
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
	case <-time.After(2 * time.Second):
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

func TestHandle_Exec_streamsStdoutLineLongerThanScannerDefaultLimit(t *testing.T) {
	t.Parallel()

	const longLineBytes = 70 * 1024 // exceeds bufio.Scanner default max token (64KiB)

	p := nosandbox.NewProvider()
	handle, err := p.Create(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer handle.Close()

	var stdout []string
	err = handle.Exec(
		context.Background(),
		"sh",
		[]string{"-c", fmt.Sprintf(
			`line=$(head -c %d /dev/zero | tr '\0' 'x'); printf '%%s\n' "$line"; echo trailer`,
			longLineBytes,
		)},
		func(line string) { stdout = append(stdout, line) },
		nil,
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	wantLong := strings.Repeat("x", longLineBytes)
	if len(stdout) != 2 {
		t.Fatalf("stdout lines = %d, want 2", len(stdout))
	}
	if stdout[0] != wantLong {
		t.Fatalf("first line length = %d, want %d", len(stdout[0]), longLineBytes)
	}
	if stdout[1] != "trailer" {
		t.Fatalf("second line = %q, want trailer", stdout[1])
	}
}
