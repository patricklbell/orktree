# .opencode Agent Notes

This directory contains OpenCode-specific agent prompts and host-side tooling
for sandboxed orchestration.

## Structure

- `opencode.yaml`: Agent and custom-tool registry
- `agents/`: Prompt files for `warden`, `orchestrator`, `worker`, `reviewer`
- `tools/`: Shell tools used by OpenCode custom tools

## Guardrails

- `warden` only dispatches orchestrators via `spawn_orchestrator`.
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