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
Think "git worktree + dev container + CoW overlay":

- The repo has a main checkout on disk.
- `janus new <branch>` creates:
  1. A **git branch** (`git worktree add -b <branch> <path>`) at a path in the
     janus data directory.
  2. An **overlayfs mount** with the git worktree as `lowerdir` and a per-worktree
     `upper` directory for CoW changes.
  3. A **Docker container** with the overlay `merged` dir mounted at `/workspace`.
- `janus switch <branch>` finds/creates the worktree and reopens the editor there.

```
repo/                         ← main checkout
  .janus/state.json           ← janus metadata

~/.local/share/janus/<repo-id>/<wt-id>/
  tree/                       ← git worktree checkout (lowerdir)
  upper/                      ← CoW writes (overlayfs upperdir)
  work/                       ← overlayfs workdir
  merged/                     ← overlayfs merged view → mounted in container
```

---

## 2. Minimal high-level components

### 2.1 CLI (`janus`)
Short, git-like commands with aliases:

```
janus init   [--source <path>] [--image <image>]
janus new    <branch> [--from <base>]         aliases: n
janus ls                                       aliases: list
janus switch <branch> [--editor <e>]          aliases: sw
janus enter  <branch>                         aliases: sh
janus exec   <branch> -- <cmd...>
janus open   <branch> [--editor <e>]
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

- The git worktree checkout path becomes the overlayfs `lowerdir`.
- Changes made inside the container appear in the overlayfs `upper` (host-visible).
- git operations (`git commit`, `git diff`, etc.) run **inside the container**
  against the real git worktree.
- `janus rm` calls `git worktree remove --force <path>` during cleanup.

---

### 2.3 Workspace storage / CoW filesystem layer (overlayfs)
```
mount -t overlay overlay \
  -o lowerdir=<git_worktree_or_source_root>,upperdir=<upper>,workdir=<work> \
  <merged>
```

- Reads from the git worktree unless a file is modified.
- Writes land in `upper` only.
- Storage per worktree proportional to changed files.

Requires `CAP_SYS_ADMIN` (run `janus` with `sudo` or use a privileged helper).

---

### 2.4 Container runtime layer
```
docker run -d \
  --name janus-<repo-id>-<wt-id> \
  -v <merged>:/workspace \
  -w /workspace \
  <image> \
  sleep infinity
```

- `janus enter <branch>` → `docker exec -it <container> bash`
- `janus exec <branch> -- <cmd>` → `docker exec <container> <cmd>`

---

### 2.5 Editor integration (`janus switch` / `janus open`)
**`janus switch <branch>`** (transparent switch):
1. Find or auto-create the worktree.
2. Ensure overlay mounted + container running.
3. Open in editor with window-reuse:
   - VS Code: attempt Dev Containers URI
     `vscode-remote://attached-container+<hex-container-name>/workspace`
     then fall back to `code --reuse-window <merged>`.
   - Other editors: `<editor> <merged>`.

**`janus open <branch>`** (no reuse):
- Opens `<merged>` in the detected or specified editor.

**Future: VS Code extension**
- Lists worktrees, calls `janus switch`, attaches Remote Containers.

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
3. Create overlay dirs (`upper/`, `work/`, `merged/`).
4. Mount overlayfs (`lowerdir` = git worktree or source root).
5. Start Docker container.
6. Persist `container_id` and `git_worktree_path`.

### 4.3 `janus switch <branch>`
1. Look up worktree by branch; if missing, auto-create via `janus new`.
2. `EnsureMounted` + `EnsureRunning`.
3. Open in editor with window reuse.

### 4.4 `janus enter <branch>`
1. `EnsureMounted` + `EnsureRunning`.
2. `docker exec -it <container> bash`.

### 4.5 `janus rm <branch>`
1. `docker stop` + `docker rm`.
2. `umount <merged>`.
3. `git worktree remove --force <tree>` (if git-backed).
4. `git worktree prune`.
5. `os.RemoveAll(<data_dir>/<id>)`.
6. Remove from state.

---

## 5. Performance
- Create: O(1) directory creation + overlay mount + container start.
- Storage: git worktree + per-worktree upper (changed files only).
- Switch: select different container + different mounted path (instant).

---

## 6. Privilege model
overlayfs requires `CAP_SYS_ADMIN`.

1. **Run with sudo** (minimal, least ergonomic): `sudo janus new main`
2. **Privileged helper** (recommended): `janus` (unprivileged) ↔ `janus-helper`
   (root) over Unix socket for mount/unmount only.
3. **Avoid overlayfs** (not recommended): full copies; loses CoW.

---

## 7. Planned roadmap

### Phase 0: CLI MVP ✓
- `init`, `new`, `ls`, `switch`, `enter`, `exec`, `open`, `rm`
- overlayfs + Docker + git worktrees
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

