# Parallel OpenCode Orchestration with orktree

This repository now supports a parallel, sandbox-first orchestration model using
OpenCode.

## High-level model

1. User asks `warden` for one or many tasks.
2. `warden` decides task decomposition and whether integration is needed.
3. `warden` issues one `spawn_orchestrator` call per independent task.
4. Each spawn creates:
   - a temporary orktree branch/workspace
   - a lightweight container with reduced privileges
   - one orchestrator run inside that sandbox
5. Each orchestrator runs `worker` + `reviewer` in an adversarial loop until
   quality gates pass.

`warden` never executes implementation commands itself.

## Why this avoids contention

- Every task uses a separate orktree branch and merged workspace.
- Spawned agents never read or write the source checkout directly.
- The container mounts only the per-run workspace and required git metadata.

## Credential and capability reduction

Spawned containers use:

- `--network none`
- `--cap-drop ALL`
- `--security-opt no-new-privileges`
- read-only root filesystem with tmpfs scratch paths
- no host home-directory mount

This keeps runs lightweight and limits accidental credential access.

## Cleanup lifecycle

- Explicit cleanup: `.opencode/tools/cleanup_run.sh --run-id <id>`
- Reaper: `.opencode/tools/reap_stale_runs.sh`

Default behavior is lossless:

- `cleanup_run.sh` removes one run's temporary container + orktree together.
- `reap_stale_runs.sh` removes stale runs and expired runs, with container + orktree cleanup together.
- Finished runs are preserved until TTL expiry so users can inspect outputs.

Use eager finished-run cleanup only when requested:

```sh
.opencode/tools/reap_stale_runs.sh --reap-finished
```

## Direct mode is still available

- Users can invoke `orchestrator` directly when sandboxing is not required.
- Users can invoke `worker` directly for standard single-agent work.

## Example

```sh
# Ensure tools are executable in your checkout (or invoke via `bash ...`).
chmod +x .opencode/tools/*.sh

# Spawn two independent orchestrators in parallel (from separate terminals)
.opencode/tools/spawn_orchestrator.sh --task "Implement API pagination"
.opencode/tools/spawn_orchestrator.sh --task "Add integration docs"

# Observe runs
.opencode/tools/list_runs.sh

# Cleanup stale runs
.opencode/tools/reap_stale_runs.sh
```