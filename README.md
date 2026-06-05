# oss-triage-agent

CLI and Go library for **AFK** (away-from-keyboard) coding agents that triage, plan, and implement work against open-source issues. The design follows a Sandcastle-style pipeline: orchestration depends on **provider interfaces** only; concrete backends (Docker, OpenCode, GitHub, and so on) live in adapters behind those seams.

Domain vocabulary, git conventions, label contracts, and package boundaries are documented in [CONTEXT.md](CONTEXT.md).

## Status

Foundation is in place with working provider adapters and an orchestrator **Run** loop:

| Area | Status |
|------|--------|
| Provider interfaces + fakes | Done |
| Orchestrator **Run** (issue load, sandbox, agent exec) | Done |
| AFK completion signal (`<promise>COMPLETE</promise>`) + idle/completion timeouts | Done |
| Managed git worktrees (`internal/git/local`) | Done |
| OpenCode agent provider (`internal/agent/opencode`) | Done |
| Host sandbox (`internal/sandbox/nosandbox`) | Done |
| Docker bind-mount sandbox (`internal/sandbox/docker`) | Done |
| `agent run` CLI (stream normalized events) | Done |
| Workflow subcommands (`triage`, `plan`, `build`) | Stubs |
| GitHub **IssueTracker** adapter | Not started |

## Requirements

- Go 1.26+ (see `go.mod`)
- `git` on `PATH` (for the local worktree adapter and debug commands)
- `opencode` on `PATH` for `agent run` (optional for library-only use)
- Docker daemon reachable from the environment for Docker sandbox tests and the `docker` adapter (optional)

## Build and test

```bash
go build -o oss-triage-agent .
go test ./...
```

CI runs `go test ./...` on push and pull requests (see [.github/workflows/ci.yml](.github/workflows/ci.yml)).

## CLI

```bash
# Workflow stubs (issue required via flag or argument)
oss-triage-agent triage --issue 42
oss-triage-agent plan --issue 42
oss-triage-agent build --issue 42

# Shorthand when the first argument is an issue id
oss-triage-agent --issue 42
```

### Agent debug command

Run an OpenCode agent and print normalized **Agent events** as JSON lines on stdout:

```bash
# Prompt via flag or stdin; requires opencode on PATH
oss-triage-agent agent run --prompt "Summarize this repo"
echo "What changed?" | oss-triage-agent agent run

# Optional OpenCode flags
oss-triage-agent agent run --model opencode/big-pickle --variant default --agent default \
  --dangerously-skip-permissions --prompt "..."
```

Set `OPENCODE_API_KEY` when the configured model requires it.

### Git worktree debug commands

Hidden maintainer commands exercise `git.Repository` against a real repo on disk without running a full AFK workflow:

```bash
# Create or reuse branch agent/issue-<N>-<slug> and worktree under .agent/worktrees/
oss-triage-agent git prepare --repo /path/to/target-repo --number 3 --title "My issue title"

# Record and read default-branch HEAD baseline (.agent/base-head)
oss-triage-agent git record-base-head --repo /path/to/target-repo
oss-triage-agent git base-head --repo /path/to/target-repo

# Check for uncommitted changes in the issue worktree
oss-triage-agent git dirty --repo /path/to/target-repo --number 3 --title "My issue title"

# Remove the worktree when clean (non-zero exit if dirty)
oss-triage-agent git remove --repo /path/to/target-repo --number 3 --title "My issue title"
```

### Git conventions (summary)

| Item | Value |
|------|--------|
| Branch | `agent/issue-<N>-<short-title>` |
| Worktrees | `.agent/worktrees/issue-<N>-<short-title>` |
| Base HEAD snapshot | `.agent/base-head` |

On failure, dirty worktrees are left in place. On success, workflows may remove a **clean** worktree when `config.Config.Git.RemoveCleanWorktreeOnSuccess` is enabled.

## AFK completion protocol

An agent signals successful completion by emitting this exact token in stdout:

```text
<promise>COMPLETE</promise>
```

The orchestrator treats this as the authoritative done signal (distinct from process exit code). Configure `RunInput.IdleTimeout` to cancel when no stdout arrives before the signal, and `RunInput.CompletionTimeout` to bound the grace period after the signal is seen. See [CONTEXT.md](CONTEXT.md#afk-completion-protocol) for the full contract.

## Package layout

| Path | Role |
|------|------|
| `cmd/` | Cobra CLI (`triage`, `plan`, `build`, `agent`, debug `git`) |
| `internal/orchestrator` | Coordinates a run via injected **Deps**; completion signal and timeouts |
| `internal/agent` | **AgentProvider**, stream **Agent events** |
| `internal/agent/opencode` | OpenCode CLI adapter (JSONL → normalized events) |
| `internal/sandbox` | **SandboxProvider**, sandbox kinds |
| `internal/sandbox/nosandbox` | Host-execution sandbox (`SandboxKind`: `none`) |
| `internal/sandbox/docker` | Docker bind-mount sandbox (`SandboxKind`: `bind-mount`) |
| `internal/issue` | **Issue**, **IssueTracker** |
| `internal/git` | **Repository** contract; `local/` and `fake/` adapters |
| `internal/workflow` | Workflow kind constants |
| `internal/prompt` | Prompt builder (stub) |
| `internal/config` | Runtime settings (workspace, git cleanup policy) |
| `internal/lifecycle` | Lifecycle phase constants (stub) |
| `internal/logging` | Structured logging contract (stub) |
| `internal/*/fake` | Test doubles for contract tests |

## Using the library

### Orchestrator

```go
o := orchestrator.New(orchestrator.Deps{
    Agent:   opencode.NewProvider("opencode/big-pickle", opencode.Options{}),
    Sandbox: nosandbox.NewProvider(),
    Issues:  tracker, // issue.IssueTracker
})

summary, err := o.Run(ctx, orchestrator.RunInput{
    IssueID:           "42",
    Workspace:         "/path/to/repo",
    IdleTimeout:       5 * time.Minute,
    CompletionTimeout: 30 * time.Second,
})
// summary.Completed, summary.Success, summary.Events, summary.TimeoutKind
```

Swap `nosandbox.NewProvider()` for a Docker provider when isolation is required:

```go
dockerProvider, err := docker.NewProvider()
handle, err := dockerProvider.Create(ctx, workspace) // workspace bind-mounted at /workspace
```

### Git worktrees

```go
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

## License

See repository license file when present.
