# SDK

Use **oss-triage-agent** as a Go library to run AFK agent workflows with your own adapters.

Domain types and contracts are defined in [CONTEXT.md](../CONTEXT.md). Package roles are summarized in [Architecture](architecture.md).

## Requirements

- Go 1.26+ (see `go.mod`)
- `git` on `PATH` when using `internal/git/local`
- `opencode` on `PATH` when using `internal/agent/opencode`
- Docker daemon when using `internal/sandbox/docker`

## Orchestrator

The orchestrator loads an issue, creates a sandbox, runs the agent, and collects normalized stream events.

```go
import (
    "context"
    "time"

    "github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
    "github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator"
    "github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/nosandbox"
)

o := orchestrator.New(orchestrator.Deps{
    Agent:   opencode.NewProvider("opencode/big-pickle", opencode.Options{}),
    Sandbox: nosandbox.NewProvider(),
    Issues:  tracker, // issue.IssueTracker
})

summary, err := o.Run(ctx, orchestrator.RunInput{
    IssueID:     "42",
    Workspace:   "/path/to/repo",
    IdleTimeout: 5 * time.Minute,
})
// summary.Completed, summary.Success, summary.Events, summary.TimeoutKind
```

### Docker sandbox

Swap `nosandbox.NewProvider()` for Docker when you need isolation:

```go
import dockersandbox "github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/docker"

dockerProvider, err := dockersandbox.NewProvider()
handle, err := dockerProvider.Create(ctx, workspace) // workspace bind-mounted at /workspace
```

## Git worktrees

```go
import (
    "github.com/harsh-m-patil/oss-triage-agent/internal/git/local"
    "github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

repo := local.New("/path/to/target-repo")

if err := repo.RecordBaseHEAD(ctx); err != nil { /* ... */ }
wt, err := repo.PrepareWorktree(ctx, issue.Issue{Number: 3, Title: "Example"})
// wt.Branch, wt.Path

dirty, _ := repo.IsDirty(ctx, iss)
if !dirty {
    _ = repo.RemoveWorktree(ctx, iss) // returns git.ErrWorktreeDirty when dirty
}
```

For tests without shelling out to git, use `internal/git/fake`.

## Issue tracker

Implement `issue.IssueTracker` or use `internal/issue/github` with a `GITHUB_TOKEN`:

```go
import issuegithub "github.com/harsh-m-patil/oss-triage-agent/internal/issue/github"

tracker, err := issuegithub.New("owner", "repo")
```

Contract tests use `internal/issue/fake`.

## Agent provider

Implement `agent.AgentProvider` or use OpenCode:

```go
provider := opencode.NewProvider("opencode/big-pickle", opencode.Options{
    Variant: "default",
    Agent:   "default",
})
argv := provider.BuildCommand("your prompt")
events, err := provider.ParseStreamLine(jsonLine)
```

## Testing

```bash
go test ./...
```

Fakes under `internal/*/fake` let you test orchestration without real Docker, OpenCode, or network I/O.
