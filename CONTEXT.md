# oss-triage-agent

CLI and library foundation for **AFK** (away-from-keyboard) coding agents that triage, plan, and build against open-source issues. Orchestration depends on **provider** interfaces only; concrete backends (Docker, OpenCode, GitHub) live behind adapters and are not imported by workflow code.

## Language

### Workflows and CLI

**AFK agent**:
A coding agent that runs with minimal human interaction, driven by prompts and streaming output.
_Avoid_: bot, autopilot, background worker (unless referring to process scheduling specifically).

**AFK workflow**:
A named end-to-end run (triage, plan, or build) that loads an issue, prepares a sandbox, and invokes an agent.
_Avoid_: job, pipeline, task (unless referring to a single schedulable unit outside this repo).

**Workflow kind** (`workflow.Kind`):
One of `triage`, `plan`, or `build`. Identifies which AFK workflow is being executed.
_Avoid_: mode, command name (the Cobra subcommand may match, but the domain concept is the workflow kind).

**oss-triage-agent**:
The root Cobra command and binary name. Unchanged across refactors.
_Avoid_: triage-agent, oss_triage_agent.

**Triage**:
Read an issue, assess it, and produce triage output (labels, comments, brief). First workflow in the Sandcastle-style sequence.
_Avoid_: review, classify (unless meaning label assignment only).

**Plan**:
Turn a triaged issue into an implementation plan.
_Avoid_: design doc, spec (unless the artifact is explicitly a written spec).

**Build**:
Execute the plan (implementation) against the repo in a sandbox.
_Avoid_: compile, ship (build here means “agent builds the change,” not “go build”).

### Providers and seams

**Provider**:
An interface that abstracts one backend concern (agent launch, sandbox lifecycle, issue tracker). Orchestration code depends on providers, not concrete implementations.
_Avoid_: driver, plugin, adapter (adapter is the concrete implementation; provider is the interface).

**Adapter**:
A concrete type that implements a provider interface (e.g. a future Docker sandbox or GitHub issue client).
_Avoid_: provider (when you mean the interface), implementation (too vague).

**Fake**:
A test adapter under `internal/*/fake` used in contract tests. Same interface as a production adapter; no real Docker, OpenCode, or network I/O.
_Avoid_: mock (unless discussing test-double taxonomy), stub (fakes may return realistic data; stubs are empty).

**Deps** (`orchestrator.Deps`):
The bundle of provider interfaces an **Orchestrator** needs: `Agent`, `Sandbox`, and `Issues`. Injected at construction; orchestration must not reach for globals or concrete packages.
_Avoid_: config, options (Deps are runtime collaborators, not static settings).

### Agent execution

**AgentProvider**:
Contract for launching an AFK coding agent: name, environment, command line, and parsing stdout into **Agent events**.
_Avoid_: runner, executor, LLM client.

**Agent event** (`agent.AgentEvent`):
A single normalized unit of agent stream output, regardless of which **AgentProvider** produced it.
_Avoid_: message, chunk, line (a raw line may yield zero or more events).

**Event kind** (`agent.EventKind`):
Discriminator on an **Agent event**: `text`, `result`, `tool_call`, `session_id`, or `usage`.
_Avoid_: type, opcode.

**Result** (`agent.Result`):
Final agent outcome attached to an event with kind `result`.
_Avoid_: response, completion.

**Tool call** (`agent.ToolCall`):
Agent invocation of a tool (name + args) on an event with kind `tool_call`.
_Avoid_: function call, MCP call (MCP may be one tool backend later).

**Usage** (`agent.Usage`):
Token or cost accounting on an event with kind `usage`.
_Avoid_: metrics, billing record.

### Sandbox

**Sandbox**:
The environment where an AFK agent runs relative to the host workspace (mounted, isolated, or none).
_Avoid_: container (a sandbox may be implemented with containers later, but the domain term is sandbox).

**SandboxProvider**:
Creates and tears down sandboxes. Returns a **Sandbox handle** for the lifetime of a run step.
_Avoid_: runtime, executor.

**Sandbox handle** (`sandbox.SandboxHandle`):
A running sandbox instance; exposes **Sandbox kind** and must be closed when the step finishes.
_Avoid_: container ID, session (session_id is agent-scoped, not sandbox-scoped).

**Sandbox kind** (`sandbox.SandboxKind`):
How the sandbox relates to the host workspace: `bind-mount` (workspace visible inside), `isolated` (copy or separate tree), or `none` (run on host with no isolation).
_Avoid_: mode, profile, tier.

### Issues and prompts

**Issue** (`issue.Issue`):
A normalized work item (number, title, body, labels) from a tracker, independent of GitHub’s API shape.
_Avoid_: ticket, GH issue (use **Issue** in code and docs).

**IssueTracker**:
Contract to read/list issues, comment, and add or remove labels.
_Avoid_: GitHub client, API wrapper.

**Issue ID**:
String identifier passed to **IssueTracker** (number or URL fragment resolved by the CLI). Used in **Run input** and `resolveIssue`.
_Avoid_: issue number (when the value may be a URL).

**Prompt builder** (`prompt.Builder`):
Renders agent prompts from **Issue** context. Stub today; owns prompt shape, not agent execution.
_Avoid_: template engine, system prompt (unless referring to a single static system string).

### Orchestration

**Orchestrator**:
Coordinates AFK workflow steps using **Deps** only. No imports of concrete Docker/OpenCode/GitHub packages.
_Avoid_: controller, service, manager.

**Run input** (`orchestrator.RunInput`):
Issue ID and workspace path for one orchestrator **Run**.
_Avoid_: request, params.

**Run summary** (`orchestrator.RunSummary`):
Observable outcomes after **Run** (issue number, agent name, **Sandbox kind**). Not a full event log.
_Avoid_: result, response.

### Supporting packages (stubs)

**Repository** (`git.Repository`):
Local git operations for workflows (clone, worktree path). Stub interface.
_Avoid_: repo, git client.

**Config** (`config.Config`):
Top-level runtime settings (e.g. workspace path). Stub struct.
_Avoid_: viper config, env (until loading is defined).

**Logger** (`logging.Logger`):
Structured info/error logging contract for workflows.
_Avoid_: slog wrapper (implementation may use slog later).

**Lifecycle phase** (`lifecycle.Phase`):
Named stage in an AFK run: `start`, `triage`, `complete`. Stub enum for future state tracking.
_Avoid_: status, step.

## Package map

| Package | Role |
|---------|------|
| `cmd` | Cobra CLI (`oss-triage-agent`, triage/plan/build subcommands) |
| `internal/agent` | **AgentProvider**, **Agent event** types |
| `internal/sandbox` | **SandboxProvider**, **Sandbox handle**, **Sandbox kind** |
| `internal/issue` | **Issue**, **IssueTracker** |
| `internal/orchestrator` | **Orchestrator**, **Deps**, **Run input**, **Run summary** |
| `internal/workflow` | **Workflow kind** constants |
| `internal/prompt` | **Prompt builder** |
| `internal/git` | **Repository** (stub) |
| `internal/config` | **Config** (stub) |
| `internal/logging` | **Logger** (stub) |
| `internal/lifecycle` | **Lifecycle phase** (stub) |
| `internal/*/fake` | **Fake** adapters for contract tests |

## Relationships

- An **Orchestrator** is constructed with **Deps** (three providers).
- One **Run** takes **Run input**, uses **IssueTracker** to load an **Issue**, **SandboxProvider** to obtain a **Sandbox handle**, and **AgentProvider** to build the agent command.
- **AgentProvider** turns stream lines into **Agent events** tagged by **Event kind**.
- **Sandbox handle** reports **Sandbox kind** until **Close**.
- **Workflow kind** (`triage` / `plan` / `build`) maps to CLI subcommands; root command with an **Issue ID** (flag or positional) shortcuts to **Triage**.
- **Fakes** implement the same interfaces as future production **Adapters**; contract tests prove **Orchestrator** needs no concrete backends.

## Example dialogue

> **Dev:** "Where does Docker fit?"
> **Maintainer:** "Behind a **SandboxProvider** **Adapter**. **Orchestrator** only sees **Deps**; it never imports Docker."
>
> **Dev:** "What's the difference between **Sandbox handle** and **Agent event** session_id?"
> **Maintainer:** "**Sandbox handle** is where the agent runs. `session_id` on an **Agent event** is the agent runtime's own session identifier from the stream."
>
> **Dev:** "Can I run `oss-triage-agent 42`?"
> **Maintainer:** "Yes — positional **Issue ID** routes to **Triage** the same as `--issue 42`, via `resolveIssue`."

## Flagged ambiguities

- **Provider** vs **Adapter**: In code comments, "provider" means the Go interface; the Docker/GitHub implementation is an **Adapter**. Fakes are adapters for tests.
- **Build** (workflow) vs `go build`: **Build** workflow means the AFK agent implements the change; it is not the Go compiler.
