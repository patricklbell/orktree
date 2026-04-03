# OpenCode Setup

This directory contains the OpenCode migration for parallel sandbox orchestration.

## Minimal migration setup

- Agents in `.opencode/opencode.yaml`: `warden`, `orchestrator`, `worker`, `reviewer`.
- `warden` is dispatch-only and fans out independent work with parallel `spawn_orchestrator` calls.
- `orchestrator` runs the `worker` + `reviewer` loop until review passes.
- `worker` and `reviewer` can still be invoked directly when sandboxing is unnecessary.

## Entry points

- Agent config: `.opencode/opencode.yaml`
- Agent prompts: `.opencode/agents/`
- Custom tools: `.opencode/tools/`

## Custom tools

- `spawn_orchestrator.sh`: creates an isolated orktree + container and starts an orchestrator run
- `list_runs.sh`: lists run state for the current repository
- `cleanup_run.sh`: removes one run's container and temporary orktree
- `reap_stale_runs.sh`: removes stale runs and expired finished runs

These scripts are intended to be executable in-place:

```sh
chmod +x .opencode/tools/*.sh
```

If your checkout did not preserve mode bits, invoke them with `bash`.

## Parallel execution

Every `spawn_orchestrator` call is independently keyed and state-isolated, so
calls can run in parallel without lock contention (except a short image-build lock).

`WARDEN_MAX_PARALLEL` can be set to cap dispatch fan-out. If unset, tools detect
parallelism from host CPU count.

## Cleanup policy

- Explicit cleanup (`cleanup_run.sh`) removes container + orktree together by default.
- Reaper cleanup removes container + orktree together for stale runs.
- Finished runs are preserved until TTL expiry by default to keep output inspection lossless.
- Use `--reap-finished` (or `WARDEN_REAP_FINISHED=1`) for eager finished-run cleanup.

## Credential isolation constraints

Spawned containers are intentionally credential-reduced:

- no host home-directory mount
- `--network none`
- `--cap-drop ALL`
- `--security-opt no-new-privileges`
- read-only root filesystem with tmpfs scratch space

## Direct agent usage

```sh
opencode run --config .opencode/opencode.yaml --agent orchestrator --prompt "<task>"
opencode run --config .opencode/opencode.yaml --agent worker --prompt "<task>"
```