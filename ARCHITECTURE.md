# Architecture: minimal Linux-only "container worktree" tool

## 0. Goal (what we're building)
A CLI tool that lets a developer (or AI agent) create multiple parallel container
workspaces over the same source directory.

- Each **workspace** gets:
  - an **isolated execution environment** (Docker container)
  - a **workspace view** of the source directory that starts identical to the base
  - **copy-on-write (CoW) file semantics**: only files changed in that workspace
    consume extra disk
- Switching between workspaces is **fast**.
- The user can:
  - list workspaces, enter a workspace, run commands in one
  - open files (via their editor) and see changes immediately
  - remove workspaces cleanly
- Linux-only, Docker allowed.
- **No version-control coupling**: the tool works with any directory.
  Diffs, branches, and commit history are the responsibility of the caller (git,
  mercurial, or any other VCS running inside the container).
- Security is *not* a concern (single-user dev tool).
- Prioritize usability and speed to implement.
- Start as a **CLI**, with a plan for future editor integrations:
  1) VS Code
  2) Vim
  3) Emacs
  4) Other editors

Non-goals for the minimal version:
- VCS awareness or built-in diff/patch commands
- Multi-tenant isolation / sandbox hardening
- Remote execution / cloud orchestration
- Cross-platform filesystem tricks (macOS/Windows host without Linux VM)
- Complex merge-conflict workflows
- Complex agent scheduling (multiple LLM agents at once can be layered later)

---

## 1. Mental model
Think "multiple writable views of one source tree, each in its own container":

- There is exactly one **base source directory** on disk (read-only in practice).
- Each workspace creates a **writable overlay** on top of the base using a kernel
  CoW mechanism.
- Each workspace runs in its own Docker container that mounts that overlay as
  `/workspace`.

This yields:
- fast create (no full copy)
- cheap storage for N workspaces
- clean separation of changes per workspace
- VCS tools (git, etc.) work normally inside the container

---

## 2. Minimal high-level components (layers)

### 2.1 CLI (what we implement)
A single executable, `agentw`, providing core commands.

Minimal command set:

```
agentw init [--source <path>] [--image <docker-image>]
```
Initialises state for a source directory. Defaults to the current directory.
Records the source path and chosen container image; does **not** require a git
repository.

```
agentw workspace new [--name <name>]
```
Creates a new workspace (overlay + container).

```
agentw workspace ls
```
Lists workspaces and their status (running/stopped).

```
agentw workspace enter <workspace_id>
```
Opens an interactive shell inside that workspace's container.

```
agentw workspace exec <workspace_id> -- <cmd...>
```
Runs a command in the workspace container (non-interactive).

```
agentw workspace open <workspace_id> [--editor code|vim|emacs|...]
```
Opens the workspace's host-visible merged path in an editor.

```
agentw workspace rm <workspace_id> [--force]
```
Removes a workspace (container + overlay dirs).

Usability notes:
- Default to sensible behaviour with minimal flags.
- Use human-friendly ids (short hash or incrementing int).
- Show clear "where is this on disk" output.
- Avoid exposing overlayfs terminology to end users.
- Intentionally omits diff/VCS commands — callers use git (or any VCS) inside
  the container.

---

### 2.2 Workspace storage / CoW filesystem layer
#### Choice: overlayfs (Linux kernel built-in)

Directories:
- Base source: supplied at `init` time — any directory, no VCS required.
- Per-workspace overlay dirs (stored in the user data directory):
  - `UPPER=<data_dir>/<workspace_id>/upper`
  - `WORK=<data_dir>/<workspace_id>/work`
  - `MERGED=<data_dir>/<workspace_id>/merged`

Mount command:
```
mount -t overlay overlay \
  -o lowerdir=<source>,upperdir=<UPPER>,workdir=<WORK> \
  <MERGED>
```

Properties:
- Reads come from the source directory unless a file has been modified.
- Writes create/modify files only in `UPPER`.
- Storage per workspace is proportional to changed files.

What we implement:
- Directory layout
- Mount/unmount lifecycle
- "is workspace mounted?" detection
- Cleanup on `rm`

What already exists:
- overlayfs in kernel
- `mount(8)` tooling

Notes:
- Mount operations require `CAP_SYS_ADMIN`; simplest approach is to run
  `agentw` with `sudo` for commands that need it, or supply a small privileged
  helper (see §7).

---

### 2.3 Container runtime layer
Each workspace gets a Docker container:
- Image: configurable (set at `init`, overridable per workspace).
- Mount: workspace `MERGED` → `/workspace` inside the container.
- Working directory: `/workspace`.
- Optional per-workspace HOME to avoid cross-workspace cache collisions.

Container lifecycle:
```
# create & start
docker run -d \
  --name agentw-<workspace_id> \
  -v <MERGED>:/workspace \
  -w /workspace \
  <image> \
  sleep infinity

# interactive shell
docker exec -it agentw-<workspace_id> bash

# non-interactive command
docker exec agentw-<workspace_id> <cmd...>

# stop & remove
docker stop agentw-<workspace_id>
docker rm   agentw-<workspace_id>
```

What we implement:
- Container naming convention
- Start / stop / inspect lifecycle
- `workspace_id → container name` mapping
- "ensure running" behaviour for commands that need it

---

### 2.4 VCS and diff (intentionally out of scope)
The tool does **not** provide built-in diff, branch, or commit commands.
Any VCS that works inside a directory works inside a workspace without special
support:
- `git diff`, `git commit`, `hg status`, etc. all work inside the container.
- The user (or agent) is responsible for initialising and using a VCS inside
  the workspace.

This keeps the tool minimal and avoids hard-coding assumptions about the
development workflow.

---

### 2.5 Editor integration layer (future; start with "open folder")
For the CLI MVP, provide a stable host path and open it.

**MVP behaviour:**
- `agentw workspace open <id>` opens `<MERGED>`:
  - For VS Code: `code <MERGED>`
  - For Vim: `vim <MERGED>`
  - For Emacs: `emacs <MERGED>`

**Future: VS Code integration**
- Extension that lists workspaces and attaches Remote - Containers to the
  running container.

**Future: Vim / Emacs integration**
- Minimal plugins that shell out to `agentw` and open terminal sessions.

---

## 3. State & metadata (minimal)
A single JSON file per initialised source directory:
- `.agentw/state.json` (in the source root, user-friendly and portable)

Global data directory for overlay dirs:
- `~/.local/share/agentw/<workspace_set_id>/` (or `/var/lib/agentw` if run
  as root)

### Schema

**Workspace-set level** (one per `init`):
```json
{
  "workspace_set_id": "<hash of source_root>",
  "source_root":      "<absolute path>",
  "image":            "<docker image>",
  "data_dir":         "<absolute path to overlay data>",
  "workspaces":       [ ... ]
}
```

**Per-workspace**:
```json
{
  "id":           "<short id>",
  "name":         "<optional human name>",
  "created_at":   "<RFC3339>",
  "container_id": "<docker container name/id or empty>"
}
```

Note: `source_root` is recorded but the tool does **not** require it to be a
git repository or any particular VCS layout.

---

## 4. Lifecycle flows

### 4.1 `init`
1. Accept `--source <path>` (default: current directory).
2. Compute `workspace_set_id` from the absolute source path (SHA-256 prefix).
3. Accept `--image <image>` (default: `ubuntu:24.04`).
4. Create `.agentw/` in the source root.
5. Write `.agentw/state.json` with the source path, image, and empty workspace
   list.
6. No VCS operations.

### 4.2 `workspace new`
1. Generate a short `workspace_id`.
2. Create overlay dirs: `<data_dir>/<workspace_id>/{upper,work,merged}`.
3. Mount overlayfs: `lowerdir=<source_root>`.
4. Start Docker container with `-v <merged>:/workspace`.
5. Record workspace metadata in `state.json`.
6. Print workspace id, merged path, and how to enter/open.

### 4.3 `workspace enter`
1. Ensure overlay is mounted; remount if needed.
2. Ensure container is running; start if needed.
3. `docker exec -it <container> bash`.

### 4.4 `workspace exec`
1. Same "ensure" steps as enter.
2. `docker exec <container> <cmd...>`.

### 4.5 `workspace open`
1. Detect or accept editor.
2. Open `<MERGED>` in the chosen editor on the host.

### 4.6 `workspace rm`
1. Stop and remove the Docker container.
2. Unmount the overlayfs.
3. Delete `<data_dir>/<workspace_id>/`.
4. Remove workspace entry from `state.json`.

---

## 5. Performance characteristics
- Create workspace: O(1) directory creation + overlay mount + container start
- Storage: source stored once + per-workspace `upper` stores only changed files
- Switch workspace: select a different container + different mounted path (instant)

---

## 6. Minimal "agent" support
For MVP, "agent = shell commands invoked via `workspace exec`" is sufficient.
The container is a stable, isolated execution environment; any agentic
framework can drive it without knowing anything about the underlying filesystem
mechanism.

Future layers can add:
- Streaming log capture
- Artifact extraction
- Multi-agent parallel execution

---

## 7. Minimal privilege model
overlayfs mounts require `CAP_SYS_ADMIN`.

Three approaches:
1. **Run the CLI with sudo** (minimal implementation) — `sudo agentw workspace new`
2. **Split into a small privileged helper** (recommended for usability) — `agentw`
   (unprivileged) communicates with `agentw-helper` (root) over a Unix socket
   for mount/unmount only.
3. **Avoid overlayfs** — fall back to full copies; loses CoW benefit.

For the minimal implementation, (1) is the smallest; (2) is the recommended next
step for ergonomics.

---

## 8. What we are not reinventing
- Any VCS (git, mercurial, etc.) — users bring their own
- Docker container lifecycle and filesystem isolation
- Linux overlayfs implementation

We only build:
- Orchestration (create/mount/start/stop/remove)
- Metadata (state.json)
- User-facing CLI

---

## 9. Planned roadmap

### Phase 0: CLI MVP
- `init`, `workspace new/ls/enter/exec/open/rm`
- overlayfs + Docker
- JSON state file
- No VCS dependency

### Phase 1: UX polish
- Human names, fuzzy selection, default editor detection
- Auto-repair stale mounts / containers
- Export / apply file patches (VCS-agnostic)

### Phase 2: VS Code integration (highest priority)
- Extension to list / switch workspaces
- Attach Remote - Containers to running container
- Open `MERGED` folder

### Phase 3: Vim + Emacs integrations
- Minimal plugins that shell out to CLI and open terminal sessions

### Phase 4: Agentic workflows
- Multi-agent parallel execution
- Per-workspace event log
- Streaming output capture

---

## 10. Glossary
- **Source**: the directory being worked on (any directory; VCS optional).
- **Base**: the source directory used as the read-only lower layer of overlayfs.
- **Workspace**: an overlay + container pairing created per agent session.
- **Upperdir**: writable layer for a workspace (stores only changed files).
- **Merged**: the mounted view combining base + upper, visible to both container
  and host editor.
- **Workspace set**: the collection of workspaces sharing one source directory.
