# Architecture

How **oss-triage-agent** is structured under the hood. For domain vocabulary and label contracts, see [CONTEXT.md](../CONTEXT.md). For using the Go library, see [SDK](sdk.md).

## Design

The CLI runs **AFK workflows** (triage → plan → build) against GitHub issues. Orchestration depends on **provider interfaces** only; concrete backends (OpenCode, Docker, GitHub) live in adapters behind those seams.

```
CLI (cmd/)
  └── workflows: triage | plan | build
        └── orchestrator.Run
              ├── IssueTracker  → GitHub REST API
              ├── SandboxProvider → docker | nosandbox
              └── AgentProvider → OpenCode CLI
```

Workflow code never imports Docker, OpenCode, or GitHub directly.

## Implementation status

| Area | Status |
|------|--------|
| Provider interfaces + fakes | Done |
| Orchestrator **Run** (issue load, sandbox, agent exec) | Done |
| Exit-based run completion + idle timeout | Done |
| Managed git worktrees (`internal/git/local`) | Done |
| OpenCode agent provider (`internal/agent/opencode`) | Done |
| Host sandbox (`internal/sandbox/nosandbox`) | Done |
| Docker bind-mount sandbox (`internal/sandbox/docker`) | Done |
| GitHub **IssueTracker** adapter (`internal/issue/github`) | Done |
| `triage` workflow CLI | Done |
| `build` workflow CLI | Done |
| `agent run` CLI (stream normalized events) | Done |
| `plan` workflow CLI | Stub |

## Run completion

The orchestrator treats a **clean agent process exit** as successful completion. `RunInput.IdleTimeout` is the guardrail for runs that stop making progress without exiting.

## Git conventions

| Item | Value |
|------|--------|
| Branch | `agent/issue-<N>-<short-title>` |
| Worktrees | `.agent/worktrees/issue-<N>-<short-title>` |
| Base HEAD snapshot | `.agent/base-head` |

On failure, dirty worktrees are left in place. On success, workflows may remove a **clean** worktree when `config.Config.Git.RemoveCleanWorktreeOnSuccess` is enabled.

Hidden `git` CLI subcommands exercise `git.Repository` for maintainer debugging without running a full workflow.

## Package layout

| Path | Role |
|------|------|
| `cmd/` | Cobra CLI (`triage`, `plan`, `build`, `agent`, debug `git`) |
| `internal/orchestrator` | Coordinates a run via injected **Deps**; exit handling and idle timeout |
| `internal/agent` | **AgentProvider**, stream **Agent events** |
| `internal/agent/opencode` | OpenCode CLI adapter (JSONL → normalized events) |
| `internal/sandbox` | **SandboxProvider**, sandbox kinds |
| `internal/sandbox/nosandbox` | Host-execution sandbox (`SandboxKind`: `none`) |
| `internal/sandbox/docker` | Docker bind-mount sandbox (`SandboxKind`: `bind-mount`) |
| `internal/issue` | **Issue**, **IssueTracker** |
| `internal/issue/github` | GitHub REST adapter |
| `internal/git` | **Repository** contract; `local/` and `fake/` adapters |
| `internal/triage` | Triage label validation and agent output parsing |
| `internal/prompt` | Workflow prompt builders (embeds triage skill docs) |
| `internal/workflow` | Workflow kind constants |
| `internal/config` | Runtime settings (workspace, git cleanup policy) |
| `internal/lifecycle` | Lifecycle phase constants (stub) |
| `internal/logging` | Structured logging for workflows |
| `internal/*/fake` | Test doubles for contract tests |

## CI

`go test ./...` runs on push and pull requests — see [.github/workflows/ci.yml](../.github/workflows/ci.yml).
