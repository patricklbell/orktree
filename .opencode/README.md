# OpenCode Setup

## Custom tools

- `spawn_orchestrator.ts`: provisions an isolated orktree branch for one task and
  records run metadata. Returns `{ run_id, branch, workspace_path }` for use in
  the subsequent Task call. Does not launch an OpenCode session — that is done by
  the warden via the Task tool.
- `list_orchestrator_runs.ts`: lists run metadata for the current repository.
- `reap_stale_runs.ts`: removes stale runs (TTL-expired) and their orktree branches.

## Dispatch flow

```
warden
 ├─ spawn_orchestrator(label="task-a") → { branch, workspace }
 ├─ spawn_orchestrator(label="task-b") → { branch, workspace }
 │   (both calls in parallel)
 ├─ Task(orchestrator, prompt includes BRANCH + WORKSPACE for task-a)
 └─ Task(orchestrator, prompt includes BRANCH + WORKSPACE for task-b)
     (both Task calls in parallel → visible as child sessions in TUI)
```

Each orchestrator Task call creates a child session the user can navigate to with
`<Leader>+Down` and cycle between with `Right`/`Left` in the OpenCode TUI.

## Isolation

Each task gets its own copy-on-write workspace via `orktree`. Only files actually
modified consume extra disk space. The warden itself should be run inside a
container if credential or filesystem isolation is required — no per-task container
is created by the tooling.

## Cleanup policy

- `reap_stale_runs` removes runs whose TTL has elapsed (default: 4 hours).
- Pass `reap_finished: true` to also remove runs that finished before their TTL.
- Orktree branches are removed alongside their run records.

## Environment

- `WARDEN_STATE_DIR`: override the default state directory (`~/.orktree-warden`).
- `WARDEN_MAX_PARALLEL`: cap dispatch fan-out (warden respects this in its prompt).
