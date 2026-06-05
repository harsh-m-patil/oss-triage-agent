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

## AFK pipeline boundaries

Sandcastle-style AFK work flows through three **Workflow kind**s in order: **Triage** → **Plan** → **Build**. Each workflow is a separate CLI invocation and orchestrator **Run**; there is no single long-lived pipeline process.

| Stage | Workflow kind | Owns | Does not own |
|-------|---------------|------|--------------|
| **Triage** | `triage` | Read **Issue**, assess fit, emit triage output (brief, comment, label updates via **IssueTracker**) | Branch/worktree creation, code changes, completion-signal handling |
| **Plan** | `plan` | Turn a triaged **Issue** into an implementation plan (prompt + agent output) | Applying labels beyond what the plan workflow defines, git writes, sandbox teardown policy |
| **Build** | `build` | Execute the plan in a **Sandbox**; agent edits the repo under git conventions below | Issue-tracker locking semantics (delegated to **IssueTracker** + label contract), choosing sandbox backend |

**Responsibility seams** (where one concern ends and another begins):

- **Orchestrator** (`orchestrator.Orchestrator`, `Run`): Loads the **Issue** through **IssueTracker**, obtains a **Sandbox handle** from **SandboxProvider**, and builds the agent command via **AgentProvider**. Future completion-signal detection (see [AFK completion protocol](#afk-completion-protocol)) lives here—not in CLI or adapters.
- **Git** (`git.Repository`, `WorktreePath()`): Local clone, branch, and worktree layout. Workflows and adapters call **Repository**; **Orchestrator** does not import concrete git commands.
- **Sandbox** (`sandbox.SandboxProvider`): Creates the environment for an agent step; **Sandbox handle** lifetime is scoped to one **Run** step unless a workflow explicitly spans steps.
- **Issue tracker** (`issue.IssueTracker`): `ReadIssue`, `Comment`, `AddLabel`, `RemoveLabel`. Category/state labels and `agent:in-progress` locking are expressed only through this interface.

**Lifecycle phase** (`lifecycle.Phase`) names coarse run stages (`start`, `triage`, `complete`) for future state tracking; workflow kind (`triage` / `plan` / `build`) names which AFK workflow is executing.

## Git conventions

One open **Issue** maps to one agent branch and one dedicated worktree:

| Convention | Value |
|------------|-------|
| Branch name | `agent/issue-<N>-<short-title>` (`<N>` = issue number; `<short-title>` = kebab-case slug from the title) |
| Worktree root | `.agent/worktrees/` (under the target repo; path surfaced by `git.Repository.WorktreePath()`) |

**Run lifecycle:**

1. Record base `HEAD` on the default branch before the agent run starts.
2. Create or reuse the branch and worktree for that issue.
3. On failure: leave a dirty worktree in place so a human or a retry can inspect or continue.
4. On success: a clean worktree may be removed; policy is adapter/workflow-specific.

Implementing worktrees and branch helpers is out of scope for this document; see the git adapter backlog. These strings are the canonical contract for downstream adapters and automation.

## AFK completion protocol

An AFK agent signals successful completion by emitting this exact token in its stdout stream:

```text
<promise>COMPLETE</promise>
```

**Orchestrator** treats this as the authoritative done signal for a workflow step. It is distinct from:

- Process exit code (agent may exit before or after the signal),
- Idle timeout (orchestrator may stop waiting without a completion signal),
- A terminal **Agent event** with kind `result` (normalized outcome, not the completion contract).

Adapters parse the raw stream; **Orchestrator** decides when a **Run** is complete based on this signal.

## GitHub label contract

Label strings below are the canonical triage and agent-locking vocabulary. Creating missing labels in the GitHub repo is a separate ops step; this section documents meaning and transition rules only.

### Roles

| Role | Cardinality | Labels |
|------|-------------|--------|
| **Category** | exactly one | `bug`, `enhancement` |
| **State** | exactly one | `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix` |
| **Lock** | optional (during implement/**Build** runs) | `agent:in-progress` |

### Meaning

- **Category** — what kind of work the issue is (`bug` vs `enhancement`). Set during or after **Triage**.
- **State** — where the issue sits in the human/agent handoff:
  - `needs-triage` — not yet assessed or needs re-triage,
  - `needs-info` — blocked on reporter or maintainer input,
  - `ready-for-agent` — approved for **Plan** / **Build** AFK workflows,
  - `ready-for-human` — agent output needs maintainer review,
  - `wontfix` — closed without implementation.
- **Lock** — `agent:in-progress` while an agent **Build** (or implement) run holds the issue; prevents concurrent agent runs on the same issue.

### Transition rules

- Every triaged issue carries **one category label and one state label** (lock is additive).
- Typical flow: `needs-triage` → (`needs-info` \| `ready-for-agent` \| `wontfix`); `needs-info` → `needs-triage` when the reporter replies with requested information.
- Adding `agent:in-progress` should accompany removing competing lock semantics; remove it when the run finishes (success, failure, or cancel).
- Maintainers may override labels manually; automation should prefer these strings.

**IssueTracker** implements add/remove; workflows must not hard-code GitHub API details.

## Maintainer sign-off (HITL gate)

The pipeline boundaries, git conventions, completion signal, and label strings in this document are **provisional** until a maintainer comments approval on [issue #2](https://github.com/harsh-m-patil/oss-triage-agent/issues/2).

Until sign-off:

- Downstream issues (e.g. GitHub **IssueTracker** adapter, automated triage) should treat this file as the draft contract but must not assume labels already exist on the repo.
- After sign-off, `CONTEXT.md` is the single source of truth; other docs and adapters should reference it rather than duplicating label lists.

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
- **Workflow kind** (`triage` / `plan` / `build`) maps to CLI subcommands; root command with an **Issue ID** (flag or positional) shortcuts to **Triage**. See [AFK pipeline boundaries](#afk-pipeline-boundaries) for stage ownership.
- **Repository** supplies `agent/issue-<N>-<short-title>` branches and `.agent/worktrees/` paths per [Git conventions](#git-conventions).
- **IssueTracker** applies the [GitHub label contract](#github-label-contract); `agent:in-progress` is the lock during **Build** runs.
- **Orchestrator** will treat `<promise>COMPLETE</promise>` as the done signal per [AFK completion protocol](#afk-completion-protocol).
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
>
> **Dev:** "When is an AFK run actually done?"
> **Maintainer:** "When the agent stream emits `<promise>COMPLETE</promise>`. **Orchestrator** watches for that—not just process exit."
>
> **Dev:** "What labels should triage set?"
> **Maintainer:** "One category (`bug` or `enhancement`) and one state (e.g. `ready-for-agent`). Full list is in the **GitHub label contract**; maintainer sign-off on issue #2 makes it canonical."

## Flagged ambiguities

- **Provider** vs **Adapter**: In code comments, "provider" means the Go interface; the Docker/GitHub implementation is an **Adapter**. Fakes are adapters for tests.
- **Build** (workflow) vs `go build`: **Build** workflow means the AFK agent implements the change; it is not the Go compiler.
