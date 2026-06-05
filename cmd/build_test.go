package cmd

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	opencodeagent "github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
	"github.com/harsh-m-patil/oss-triage-agent/internal/git"
	issuepkg "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/nosandbox"
)

func TestRunBuildWorkflow_locksRunsCommentsAndUnlocks(t *testing.T) {
	t.Parallel()

	tracker := &recordingIssueTracker{
		issue: issuepkg.Issue{
			Number: 9,
			Title:  "Wire build workflow",
			Body:   "Implement the end-to-end build command.",
		},
	}
	worktreePath := t.TempDir()
	repo := &recordingRepository{
		worktree: git.Worktree{
			Path:   worktreePath,
			Branch: "agent/issue-9-wire-build-workflow",
		},
	}
	agentProvider := &recordingBuildAgent{}

	summary, err := runBuildWorkflow(context.Background(), buildWorkflowDeps{
		Issues:  tracker,
		Repo:    repo,
		Sandbox: nosandbox.NewProvider(),
		Agent:   agentProvider,
		Prompt:  prompt.Builder{},
	}, buildOptions{IssueID: "9"})
	if err != nil {
		t.Fatalf("runBuildWorkflow: %v", err)
	}

	if !summary.Success || !summary.Completed {
		t.Fatalf("summary = %+v, want successful completed run", summary)
	}
	if !slices.Equal(repo.ops, []string{"record-base-head", "prepare-worktree"}) {
		t.Fatalf("repo ops = %v", repo.ops)
	}
	if !slices.Equal(tracker.ops, []string{"read", "lock", "comment", "unlock"}) {
		t.Fatalf("tracker ops = %v", tracker.ops)
	}
	if len(tracker.comments) != 1 || tracker.comments[0] == "" {
		t.Fatalf("comments = %v, want one non-empty comment", tracker.comments)
	}
	for _, want := range []string{
		"Build succeeded",
		"Completed: `true`",
		"Branch: `agent/issue-9-wire-build-workflow`",
	} {
		if !strings.Contains(tracker.comments[0], want) {
			t.Fatalf("comment missing %q:\n%s", want, tracker.comments[0])
		}
	}
	for _, want := range []string{
		"Wire build workflow",
		"Implement the end-to-end build command.",
	} {
		if !strings.Contains(agentProvider.prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, agentProvider.prompt)
		}
	}
}

func TestRunBuildWorkflow_commentsAndUnlocksOnFailure(t *testing.T) {
	t.Parallel()

	tracker := &recordingIssueTracker{
		issue: issuepkg.Issue{
			Number: 9,
			Title:  "Wire build workflow",
			Body:   "Implement the end-to-end build command.",
		},
	}
	repo := &recordingRepository{
		worktree: git.Worktree{
			Path:   t.TempDir(),
			Branch: "agent/issue-9-wire-build-workflow",
		},
	}

	_, err := runBuildWorkflow(context.Background(), buildWorkflowDeps{
		Issues:  tracker,
		Repo:    repo,
		Sandbox: failingSandboxProvider{err: fmt.Errorf("sandbox exploded")},
		Agent:   &recordingBuildAgent{},
		Prompt:  prompt.Builder{},
	}, buildOptions{IssueID: "9"})
	if err == nil {
		t.Fatal("runBuildWorkflow: want error, got nil")
	}
	if !strings.Contains(err.Error(), "sandbox exploded") {
		t.Fatalf("err = %v, want sandbox failure", err)
	}
	if !slices.Equal(tracker.ops, []string{"read", "lock", "comment", "unlock"}) {
		t.Fatalf("tracker ops = %v", tracker.ops)
	}
	if len(tracker.comments) != 1 {
		t.Fatalf("comments = %v, want one failure comment", tracker.comments)
	}
	if !strings.Contains(tracker.comments[0], "failed") {
		t.Fatalf("comment = %q, want failure summary", tracker.comments[0])
	}
	if !strings.Contains(tracker.comments[0], "sandbox exploded") {
		t.Fatalf("comment = %q, want failure details", tracker.comments[0])
	}
}

func TestRunBuildWorkflow_includesAgentStderrInFailureComment(t *testing.T) {
	t.Parallel()

	tracker := &recordingIssueTracker{
		issue: issuepkg.Issue{
			Number: 9,
			Title:  "Wire build workflow",
			Body:   "Implement the end-to-end build command.",
		},
	}
	repo := &recordingRepository{
		worktree: git.Worktree{
			Path:   t.TempDir(),
			Branch: "agent/issue-9-wire-build-workflow",
		},
	}

	_, err := runBuildWorkflow(context.Background(), buildWorkflowDeps{
		Issues:  tracker,
		Repo:    repo,
		Sandbox: nosandbox.NewProvider(),
		Agent:   stderrFailingBuildAgent{},
		Prompt:  prompt.Builder{},
	}, buildOptions{IssueID: "9"})
	if err == nil {
		t.Fatal("runBuildWorkflow: want error, got nil")
	}
	if len(tracker.comments) != 1 {
		t.Fatalf("comments = %v, want one failure comment", tracker.comments)
	}
	for _, want := range []string{"stderr tail", "permission denied", "opencode"} {
		if !strings.Contains(tracker.comments[0], want) {
			t.Fatalf("comment missing %q:\n%s", want, tracker.comments[0])
		}
	}
}

func TestRunBuildWorkflow_logsPhasesAndAgentProgress(t *testing.T) {
	t.Parallel()

	tracker := &recordingIssueTracker{
		issue: issuepkg.Issue{
			Number: 9,
			Title:  "Wire build workflow",
			Body:   "Implement the end-to-end build command.",
		},
	}
	repo := &recordingRepository{
		worktree: git.Worktree{
			Path:   t.TempDir(),
			Branch: "agent/issue-9-wire-build-workflow",
		},
	}
	var logs bytes.Buffer

	_, err := runBuildWorkflow(context.Background(), buildWorkflowDeps{
		Issues:  tracker,
		Repo:    repo,
		Sandbox: nosandbox.NewProvider(),
		Agent:   loggingBuildAgent{},
		Prompt:  prompt.Builder{},
		Log:     &logs,
	}, buildOptions{IssueID: "9"})
	if err != nil {
		t.Fatalf("runBuildWorkflow: %v", err)
	}

	for _, want := range []string{
		"[build] reading issue 9",
		"[build] locking issue #9",
		"[build] recording base HEAD",
		"[build] preparing worktree agent/issue-9-wire-build-workflow",
		"[build] worktree ready:",
		"[build] starting agent recording-log-agent in",
		"[build] agent command: sh -c",
		"[build] agent session: sess_123",
		"[build] agent text: thinking through slog migration",
		"[build] agent result: thinking through slog migration",
		"[build] agent tool: bash npm test",
		"[build] agent stderr: permission denied",
		"[build] agent finished: completed=true success=true",
		"[build] posting issue comment for #9",
		"[build] unlocking issue #9",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs missing %q:\n%s", want, logs.String())
		}
	}
}

type recordingIssueTracker struct {
	issue    issuepkg.Issue
	ops      []string
	comments []string
}

func (t *recordingIssueTracker) ReadIssue(_ context.Context, id string) (*issuepkg.Issue, error) {
	if id != "9" {
		return nil, fmt.Errorf("ReadIssue id = %q, want 9", id)
	}
	t.ops = append(t.ops, "read")
	iss := t.issue
	return &iss, nil
}

func (t *recordingIssueTracker) ListIssues(context.Context, string) ([]issuepkg.Issue, error) {
	return nil, fmt.Errorf("unexpected ListIssues call")
}

func (t *recordingIssueTracker) Comment(_ context.Context, id, body string) error {
	if id != "9" {
		return fmt.Errorf("Comment id = %q, want 9", id)
	}
	t.ops = append(t.ops, "comment")
	t.comments = append(t.comments, body)
	return nil
}

func (t *recordingIssueTracker) AddLabel(context.Context, string, string) error {
	return fmt.Errorf("unexpected AddLabel call")
}

func (t *recordingIssueTracker) RemoveLabel(context.Context, string, string) error {
	return fmt.Errorf("unexpected RemoveLabel call")
}

func (t *recordingIssueTracker) Lock(_ context.Context, id string) error {
	if id != "9" {
		return fmt.Errorf("Lock id = %q, want 9", id)
	}
	t.ops = append(t.ops, "lock")
	return nil
}

func (t *recordingIssueTracker) Unlock(_ context.Context, id string) error {
	if id != "9" {
		return fmt.Errorf("Unlock id = %q, want 9", id)
	}
	t.ops = append(t.ops, "unlock")
	return nil
}

type recordingRepository struct {
	worktree git.Worktree
	ops      []string
}

func (r *recordingRepository) Clone(context.Context, string, string) error {
	return fmt.Errorf("unexpected Clone call")
}

func (r *recordingRepository) WorktreePath() string {
	return ".agent/worktrees"
}

func (r *recordingRepository) BranchName(iss issuepkg.Issue) string {
	return git.BranchName(iss)
}

func (r *recordingRepository) PrepareWorktree(_ context.Context, iss issuepkg.Issue) (git.Worktree, error) {
	if iss.Number != 9 {
		return git.Worktree{}, fmt.Errorf("PrepareWorktree issue = %d, want 9", iss.Number)
	}
	r.ops = append(r.ops, "prepare-worktree")
	return r.worktree, nil
}

func (r *recordingRepository) RecordBaseHEAD(context.Context) error {
	r.ops = append(r.ops, "record-base-head")
	return nil
}

func (r *recordingRepository) BaseHEAD(context.Context) (string, error) {
	return "", fmt.Errorf("unexpected BaseHEAD call")
}

func (r *recordingRepository) IsDirty(context.Context, issuepkg.Issue) (bool, error) {
	return false, fmt.Errorf("unexpected IsDirty call")
}

func (r *recordingRepository) RemoveWorktree(context.Context, issuepkg.Issue) error {
	return fmt.Errorf("unexpected RemoveWorktree call")
}

type recordingBuildAgent struct {
	prompt string
}

func (a *recordingBuildAgent) Name() string { return "recording-build-agent" }

func (a *recordingBuildAgent) Env() map[string]string { return nil }

func (a *recordingBuildAgent) BuildCommand(prompt string) []string {
	a.prompt = prompt
	return []string{
		"sh", "-c",
		`true`,
	}
}

func (a *recordingBuildAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

type failingSandboxProvider struct {
	err error
}

func (p failingSandboxProvider) Create(context.Context, string) (sandbox.SandboxHandle, error) {
	return failingSandboxHandle{err: p.err}, nil
}

type failingSandboxHandle struct {
	err error
}

func (h failingSandboxHandle) Kind() sandbox.SandboxKind { return sandbox.SandboxNone }

func (h failingSandboxHandle) WorkspacePath() string { return "" }

func (h failingSandboxHandle) Exec(context.Context, string, []string, map[string]string, func(string), func(string)) error {
	return h.err
}

func (h failingSandboxHandle) Close() error { return nil }

type stderrFailingBuildAgent struct{}

func (stderrFailingBuildAgent) Name() string { return "opencode" }

func (stderrFailingBuildAgent) Env() map[string]string { return nil }

func (stderrFailingBuildAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`echo 'permission denied' >&2; exit 17`,
	}
}

func (stderrFailingBuildAgent) ParseStreamLine(string) ([]agent.AgentEvent, error) {
	return nil, nil
}

type loggingBuildAgent struct{}

func (loggingBuildAgent) Name() string { return "recording-log-agent" }

func (loggingBuildAgent) Env() map[string]string { return nil }

func (loggingBuildAgent) BuildCommand(string) []string {
	return []string{
		"sh", "-c",
		`printf '%s\n' '{"type":"step_start","sessionID":"sess_123"}'; ` +
			`printf '%s\n' '{"type":"text","part":{"type":"text","text":"thinking through slog migration"}}'; ` +
			`printf '%s\n' '{"type":"tool_use","part":{"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"npm test"}}}}'; ` +
			`echo 'permission denied' >&2`,
	}
}

func (loggingBuildAgent) ParseStreamLine(line string) ([]agent.AgentEvent, error) {
	return opencodeagent.NewProvider("opencode/test", opencodeagent.Options{}).ParseStreamLine(line)
}

var (
	_ issuepkg.IssueTracker = (*recordingIssueTracker)(nil)
	_ git.Repository        = (*recordingRepository)(nil)
	_ agent.AgentProvider   = (*recordingBuildAgent)(nil)
	_ agent.AgentProvider   = stderrFailingBuildAgent{}
	_ agent.AgentProvider   = loggingBuildAgent{}
	_ sandbox.SandboxProvider = nosandbox.NewProvider()
	_ sandbox.SandboxProvider = failingSandboxProvider{}
)
