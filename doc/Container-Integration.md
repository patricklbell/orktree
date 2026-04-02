# Container Workflows — Running Orktrees in Docker and Podman

> **Linux only.** Orktree requires fuse-overlayfs, which is Linux-only.
> The workflows below apply to Docker or Podman running natively on Linux.

## Prerequisites

Enable `user_allow_other` in the FUSE configuration so the Docker daemon can
access orktree mount points:

```bash
echo 'user_allow_other' | sudo tee -a /etc/fuse.conf
```

Run `orktree doctor` to verify the setting is active.

---

## How orktree + containers interact

Each orktree's `.git` file contains an absolute host path pointing to git
worktree metadata (e.g., `gitdir: /home/user/repo/.git/worktrees/tree`).
Submodule `.git` files likewise reference host-absolute paths. For git to work
inside a container, these paths must resolve — which means the repository and
its `.orktree` sibling directory must be mounted at their **real host paths**,
not at a synthetic `/workspace`.

---

## Quick start

```bash
srcroot="$(orktree path -)" || exit 1
wspath="$(orktree path feature-x)" || exit 1
docker run --rm -it \
  -v "$srcroot":"$srcroot" \
  -v "$srcroot.orktree":"$srcroot.orktree" \
  -w "$wspath" myimage bash
```

`orktree path -` prints the source root. Appending `.orktree` gives the sibling
data directory. Both are mounted at their host paths so every absolute path
inside `.git` files, worktree metadata, and submodule pointers resolves
correctly.

> **Warning:** Do not run `orktree rm` while a container is using the orktree.
> The overlay will be unmounted, causing I/O errors or data loss in the running
> container. Stop the container first.

> **Common mistake:** If you omit the `.orktree` mount, git commands inside the
> container will fail with `fatal: not a git repository` because the overlay
> internals are invisible. Always mount both the repo and its `.orktree`
> sibling.

The container's working directory will be the orktree's host path (e.g.,
`/home/user/repo.orktree/feature-x/`).

---

## UID mapping

Files created inside a container are owned by the container's UID. If running
as root inside the container (the default), files in the orktree upper dir will
be owned by root on the host.

### Docker

Pass `--user` to match the host UID:

```bash
docker run --rm -it --user "$(id -u):$(id -g)" \
  -v "$srcroot":"$srcroot" \
  -v "$srcroot.orktree":"$srcroot.orktree" \
  -w "$wspath" myimage
```

### Podman (recommended)

Podman maps the host UID automatically with `--userns=keep-id`:

```bash
podman run --rm -it --userns=keep-id \
  -v "$srcroot":"$srcroot" \
  -v "$srcroot.orktree":"$srcroot.orktree" \
  -w "$wspath" myimage
```

---

## SELinux

On SELinux-enabled hosts (Fedora, RHEL), add `:z` to both volume mounts:

```bash
docker run --rm -it \
  -v "$srcroot":"$srcroot":z \
  -v "$srcroot.orktree":"$srcroot.orktree":z \
  -w "$wspath" myimage
```

---

## Parallel containers

Use `orktree ls --quiet` to spawn one container per orktree:

```bash
srcroot="$(orktree path -)" || exit 1
for branch in $(orktree ls --quiet); do
  wspath="$(orktree path "$branch")" || continue
  docker run -d --name "dev-${branch//\//-}" \
    -v "$srcroot":"$srcroot" \
    -v "$srcroot.orktree":"$srcroot.orktree" \
    -w "$wspath" myimage
done
```

---

## docker-compose

Generate an `.env` file containing the resolved paths, then run
`docker compose up`. Docker Compose loads `.env` automatically for YAML
interpolation.

```bash
srcroot="$(orktree path -)"
cat > .env <<EOF
SRC_ROOT=$srcroot
ORKTREE_DIR=$srcroot.orktree
WORKSPACE_PATH=$(orktree path feature-x)
EOF
docker compose up
```

> **Note:** Regenerate `.env` whenever you switch to a different orktree.

`docker-compose.yml`:

```yaml
services:
  dev:
    image: myimage
    volumes:
      - type: bind
        source: ${SRC_ROOT:?Set SRC_ROOT}
        target: ${SRC_ROOT}
      - type: bind
        source: ${ORKTREE_DIR:?Set ORKTREE_DIR}
        target: ${ORKTREE_DIR}
    working_dir: ${WORKSPACE_PATH:?Set WORKSPACE_PATH}
    stdin_open: true
    tty: true
```

---

## VS Code devcontainer

Set the source root as an environment variable, then open the orktree:

```bash
export ORKTREE_SRC_ROOT="$(orktree path -)"
code "$(orktree path feature-x)"
```

`.devcontainer/devcontainer.json`:

```jsonc
{
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "workspaceMount": "source=${localWorkspaceFolder},target=${localWorkspaceFolder},type=bind",
  "workspaceFolder": "${localWorkspaceFolder}",
  "mounts": [
    "source=${localEnv:ORKTREE_SRC_ROOT},target=${localEnv:ORKTREE_SRC_ROOT},type=bind",
    "source=${localEnv:ORKTREE_SRC_ROOT}.orktree,target=${localEnv:ORKTREE_SRC_ROOT}.orktree,type=bind"
  ]
}
```

`${localEnv:ORKTREE_SRC_ROOT}` reads the host environment variable set before
launching VS Code. Both the source root and its `.orktree` sibling are mounted
at their host paths so git works inside the devcontainer.

For simple file editing without git support, skip the extra mounts and open the
merged path directly:

```bash
code "$(orktree path feature-x)"
```

---

## Lifecycle and caveats

- The fuse-overlayfs mount must stay active while the container runs.
- `orktree rm` has no way to detect active bind-mounts from containers, so
  this is a user responsibility.
- **Docker-in-Docker:** if you're already inside a container, the bind-mount
  paths must be valid on the Docker host, not inside the outer container.
- **CI/CD:** these patterns work identically in CI — use `orktree path` in
  your pipeline scripts.
