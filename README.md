# oss-triage-agent

CLI and Go library for **AFK** (away-from-keyboard) coding agents that triage, plan, and implement work against open-source issues. The design follows a Sandcastle-style pipeline: orchestration depends on **provider interfaces** only; concrete backends (Docker, OpenCode, GitHub, and so on) live in adapters behind those seams.

Domain vocabulary, git conventions, label contracts, and package boundaries are documented in [CONTEXT.md](CONTEXT.md).

## Status

Early foundation: provider interfaces, fakes, orchestrator skeleton, and a **managed git worktree** layer are in place. Workflow subcommands (`triage`, `plan`, `build`) are stubs; full AFK runs and GitHub integration are still being built out.

## Requirements

- Go 1.26+ (see `go.mod`)
- `git` on `PATH` (for the local worktree adapter and debug commands)

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

## Package layout

| Path | Role |
|------|------|
| `cmd/` | Cobra CLI (`triage`, `plan`, `build`, debug `git`) |
| `internal/orchestrator` | Coordinates a run via injected **Deps** |
| `internal/agent` | **AgentProvider**, stream **Agent events** |
| `internal/sandbox` | **SandboxProvider**, sandbox kinds |
| `internal/issue` | **Issue**, **IssueTracker** |
| `internal/git` | **Repository** contract; `local/` and `fake/` adapters |
| `internal/workflow` | Workflow kind constants |
| `internal/config` | Runtime settings (workspace, git cleanup policy) |
| `internal/*/fake` | Test doubles for contract tests |

## Using the git library

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
