# orktree Wiki

Welcome to the orktree wiki — guides for advanced usage and integration.

OpenCode migration note: the minimal orchestration setup is `warden` + `orchestrator` + `worker` + `reviewer` in `.opencode/opencode.yaml`, with `warden` dispatching independent tasks via parallel `spawn_orchestrator` calls. Cleanup is lossless by default (finished runs are kept until TTL) with optional eager prune via `reap_stale_runs.sh --reap-finished`, and direct `orchestrator`/`worker` use remains supported.

## Pages

- [Build Tool Integration](Build-Tool-Integration.md) — working with CMake and other build tools in orktrees
- [Shell Integration](Shell-Integration.md) — setting up cd-on-switch for bash, zsh, fish, and POSIX sh
- [Container Workflows](Container-Integration.md) — running orktrees in Docker, Podman, and devcontainers
- [Parallel OpenCode Orchestration](../docs/Parallel-Orchestration-With-Orktree.md) — sandboxed parallel agent runs with warden + orktree
