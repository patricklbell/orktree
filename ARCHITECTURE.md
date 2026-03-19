# Architecture: minimal Linux-only "container worktree" tool

## 0. Goal (what we're building)
A CLI tool (`janus`) that lets a developer (or AI agent) create multiple parallel
container worktrees over the same repository.

- Each **worktree** gets:
  - an **isolated execution environment** (Docker container)
  - its own **git branch** (created via `git worktree add`)
  - a **copy-on-write (CoW) overlay** on top of the git worktree checkout
    so that only changed files consume extra disk
- Switching between worktrees is **fast** — `janus switch <branch>` starts
  the worktree if needed and reopens it in the editor transparently.
- The user can:
  - list worktrees, enter a worktree shell, run commands in one
  - open files (via their editor) and see changes immediately
  - switch between branches as if switching between contexts
- Linux-only, Docker required, git required for worktree branch tracking.
- Security is *not* a concern (single-user dev tool).
- Prioritize usability and speed to implement.
- CLI-first, with a plan for future editor integrations:
  1) VS Code (highest priority)
  2) Vim
  3) Emacs

Non-goals for the minimal version:
- Multi-tenant isolation / sandbox hardening
- Remote execution / cloud orchestration
- Cross-platform filesystem tricks (macOS/Windows host without Linux VM)
- Complex merge-conflict workflows beyond standard git tooling
- Complex agent scheduling (multiple LLM agents at once can be layered later)

---

## 1. Mental model
Think "git worktree + dev container, direct bind-mount":

- The repo has a main checkout on disk.
- `janus new <branch>` creates:
  1. A **git branch** (`git worktree add -b <branch> <path>`) at a path in the
     janus data directory.
  2. A **Docker container** with the git worktree checkout bind-mounted at
     `/workspace`.
- `janus switch <branch>` finds/creates the worktree and reopens the editor there.

```
repo/                         ← main checkout
  .janus/state.json           ← janus metadata

~/.local/share/janus/<repo-id>/<wt-id>/
  tree/                       ← git worktree checkout → bind-mounted in container
```

No overlayfs, no elevated privileges.  The only requirement is membership in the
**`docker` group**.

---

## 2. Minimal high-level components

### 2.1 CLI (`janus`)
Short, git-like commands with aliases:

```
janus init   [--source <path>] [--image <image>]
janus new    <branch> [--from <base>]         aliases: n
janus ls                                       aliases: list
janus switch <branch>                          aliases: sw
janus enter  <branch>                         aliases: sh
janus exec   <branch> -- <cmd...>
janus rm     <branch> [--force]               aliases: remove
```

Design principles:
- Primary identifier for a worktree is its **branch name** (not an opaque ID).
- IDs/prefixes also accepted for all ref arguments.
- `janus switch` is the main workflow command — it auto-creates the worktree if
  it doesn't exist yet.

---

### 2.2 Git integration
Each worktree is backed by a real git worktree:

```
git worktree add -b <branch> <data_dir>/<id>/tree [<from>]
```

- The git worktree checkout is bind-mounted directly into the container.
- Changes made inside the container go straight to the git worktree — normal
  `git commit`, `git diff`, etc. work as expected.
- `janus rm` calls `git worktree remove --force <path>` during cleanup.

---

### 2.3 Container runtime layer
```
docker run -d \
  --name janus-<repo-id>-<wt-id> \
  -v <git_worktree_or_source_root>:/workspace \
  -w /workspace \
  <image> \
  sleep infinity
```

- `janus enter <branch>` → `docker exec -it <container> bash`
- `janus exec <branch> -- <cmd>` → `docker exec <container> <cmd>`

---

### 2.5 Editor integration (`janus switch`)
`janus switch <branch>` ensures the worktree is running and then attempts to
reopen **VS Code** inside the container using the Dev Containers
"Attach to Running Container" URI:

```
vscode-remote://attached-container+<hex-json>/workspace
```

where `<hex-json>` is the hex encoding of `{"containerName":"/<container>"}`.

This is the only supported editor integration because it is the only one that
can perform a true "reopen in container" in a single command. Vim, Emacs, and
other editors do not have an equivalent workflow, so no integration is
attempted for them — the user can use `janus enter <branch>` to get a shell
inside any worktree's container instead.

If VS Code is not installed or the Dev Containers extension is missing, `janus
switch` still starts the worktree and prints the container name so the user can
attach manually.

**Future: VS Code extension**
- Lists worktrees, calls `janus switch`, manages the Dev Containers lifecycle.

---

## 3. State & metadata

`.janus/state.json` in the source root:

```json
{
  "id": "<sha256-prefix-of-source-root>",
  "source_root": "<absolute-path>",
  "is_git_repo": true,
  "image": "<docker-image>",
  "data_dir": "~/.local/share/janus/<id>",
  "worktrees": [
    {
      "id": "<random-hex>",
      "branch": "<git-branch-name>",
      "git_worktree_path": "<data_dir>/<id>/tree",
      "container_id": "janus-<repo-id>-<wt-id>",
      "created_at": "<RFC3339>"
    }
  ]
}
```

---

## 4. Lifecycle flows

### 4.1 `janus init`
1. Resolve source root (default: cwd).
2. Detect git repo (`git rev-parse --git-dir`); set `is_git_repo`.
3. Accept `--image` (default `ubuntu:24.04`).
4. Write `.janus/state.json`.

### 4.2 `janus new <branch>`
1. Create state entry with random `id` and given `branch`.
2. If `is_git_repo`: run `git worktree add [-b] <tree_path> <branch> [<from>]`.
3. Start Docker container with `<tree_path>` (or source root) bind-mounted at
   `/workspace`.
4. Persist `container_id` and `git_worktree_path`.

### 4.3 `janus switch <branch>`
1. Look up worktree by branch; if missing, auto-create via `janus new`.
2. `EnsureRunning`.
3. Attempt VS Code Dev Containers `attached-container` URI; print container
   info if VS Code is not available.

### 4.4 `janus enter <branch>`
1. `EnsureRunning`.
2. `docker exec -it <container> bash`.

### 4.5 `janus rm <branch>`
1. `docker stop` + `docker rm`.
2. `git worktree remove --force <tree>` (if git-backed).
3. `git worktree prune`.
4. `os.RemoveAll(<data_dir>/<id>)`.
5. Remove from state.

---

## 5. Performance
- Create: O(1) git worktree add + container start.
- Storage: git worktree checkout per branch (shared objects via git).
- Switch: select different container + different bind-mounted path (instant).

---

## 6. Privilege model
No elevated privileges are required.  The only group membership needed is
**`docker`**, which allows running Docker commands as a normal user:

```sh
sudo usermod -aG docker $USER   # log out and back in to apply
```

Run `janus setup` to verify all prerequisites are met.

---

## 7. Planned roadmap

### Phase 0: CLI MVP ✓
- `setup`, `init`, `new`, `ls`, `switch`, `enter`, `exec`, `rm`
- Direct bind-mount + Docker + git worktrees (no overlayfs, no sudo)
- JSON state file

### Phase 1: UX polish
- Auto-repair stale mounts/containers
- Fuzzy branch-name selection
- Export/apply file patches (VCS-agnostic)

### Phase 2: VS Code integration (highest priority)
- Extension to list/switch worktrees
- Auto-attach Remote Containers to running container
- Show git diff per worktree

### Phase 3: Vim + Emacs integrations
- Minimal plugins that shell out to `janus` and open terminal sessions

### Phase 4: Agentic workflows
- Multi-agent parallel execution
- Per-worktree event log
- Streaming output capture

---

## 8. Glossary
- **Repo**: the project being worked on (must be a git repo for full features).
- **Worktree**: a git branch + overlayfs overlay + Docker container triple.
- **Git worktree**: the git-managed directory at `<data_dir>/<id>/tree`.
- **Upperdir**: writable CoW layer (stores only changed files).
- **Merged**: the overlayfs unified view mounted inside the container.

