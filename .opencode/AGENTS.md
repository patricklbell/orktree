# .opencode Agent Notes

This directory contains OpenCode-specific agent prompts and host-side tooling
for sandboxed orchestration.

## Minimal setup

- Register four agents in `.opencode/opencode.yaml`: `warden`, `orchestrator`, `worker`, `reviewer`.
- Register custom tools: `spawn_orchestrator`, `list_orchestrator_runs`, `reap_stale_runs`.
- Keep `warden` dispatch-only: it should orchestrate by tool call, not by direct implementation.

## Structure

- `opencode.yaml`: Agent and custom-tool registry
- `agents/`: Prompt files for `warden`, `orchestrator`, `worker`, `reviewer`
- `tools/`: Shell tools used by OpenCode custom tools

## Guardrails

- `warden` only dispatches orchestrators via `spawn_orchestrator`.
- Independent tasks should be fanned out with parallel `spawn_orchestrator` calls.
- Spawned orchestrators run in isolated orktrees and lightweight containers.
- Containers only mount:
  - the spawned orktree workspace
  - source `.git` directory
  - worktree git metadata directory
- No host home directory, credentials, or source checkout files are mounted.

## Maintenance

- Keep scripts POSIX/Bash compatible and ASCII-only.
- Keep `.opencode/tools/*.sh` executable.
- Keep state under `${HOME}/.orktree-warden` unless overridden.
- Reaper behavior must stay idempotent and safe to run frequently.
- Preserve finished runs by default; only use eager finished-run cleanup when explicitly configured.

## Direct usage

- Use `orchestrator` directly when sandboxing is not required.
- Use `worker` directly for normal single-agent implementation tasks.