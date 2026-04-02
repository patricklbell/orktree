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

## The problem

Each orktree is an isolated workspace. But build environments, language
runtimes, and system dependencies differ across projects and branches.
Containers let you pair each orktree with a consistent, reproducible
environment — all sharing the same base image. No rebuild needed per branch.

---

## Quick start

Resolve the workspace path and bind-mount it into a container:

```bash
# Resolve the workspace path (exits on error)
wspath="$(orktree path feature-x)" || exit 1
docker run --rm -it -v "$wspath":/workspace -w /workspace myimage bash
```

This works because `orktree path` returns the fuse-overlayfs merged view,
which is a regular directory. Any container runtime can bind-mount it. Writes
inside the container land in the orktree's CoW upper directory.

> **Warning:** Do not run `orktree rm` while a container is using the orktree.
> The overlay will be unmounted, causing I/O errors or data loss in the running
> container. Stop the container first.

---

## UID mapping

Files created inside a container are owned by the container's UID. If running
as root inside the container (the default), files in the orktree upper dir will
be owned by root on the host.

### Docker

Pass `--user` to match the host UID:

```bash
docker run --rm -it --user "$(id -u):$(id -g)" -v "$wspath":/workspace myimage
```

### Podman (recommended)

Podman maps the host UID automatically with `--userns=keep-id`:

```bash
podman run --rm -it --userns=keep-id -v "$wspath":/workspace myimage
```

---

## SELinux

On SELinux-enabled hosts (Fedora, RHEL), add `:z` to the volume mount so the
container can access the bind-mounted directory:

```bash
docker run --rm -it -v "$wspath":/workspace:z myimage
```

---

## Parallel containers

Use `orktree ls --quiet` to spawn one container per orktree:

```bash
for branch in $(orktree ls --quiet); do
  wspath="$(orktree path "$branch")" || continue
  docker run -d --name "dev-${branch//\//-}" -v "$wspath":/workspace myimage
done
```

---

## docker-compose

Generate an `.env` file containing the resolved workspace path, then run
`docker compose up`. Docker Compose loads `.env` automatically for YAML
interpolation.

```bash
echo "WORKSPACE_PATH=$(orktree path feature-x)" > .env
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
        source: ${WORKSPACE_PATH:?Set WORKSPACE_PATH}
        target: /workspace
    working_dir: /workspace
    stdin_open: true
    tty: true
```

---

## VS Code devcontainer

Use `workspaceMount` to point at the orktree merged view:

```jsonc
{
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "workspaceMount": "source=${localWorkspaceFolder},target=/workspace,type=bind",
  "workspaceFolder": "/workspace"
}
```

If you open the merged path directly in VS Code (`code "$(orktree path
feature-x)"`), the default mount just works — no custom `workspaceMount`
needed.

---

## Lifecycle and caveats

- The fuse-overlayfs mount must stay active while the container runs.
- `orktree rm` has no way to detect active bind-mounts from containers, so
  this is a user responsibility.
- **Docker-in-Docker:** if you're already inside a container, the bind-mount
  path must be valid on the Docker host, not inside the outer container.
- **CI/CD:** these patterns work identically in CI — use `orktree path` in
  your pipeline scripts.
