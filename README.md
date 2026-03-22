# janus
Container worktrees for agents — each branch gets its own isolated workspace
with **no file duplication** (copy-on-write via `fuse-overlayfs`).

`janus` creates a git worktree + fuse-overlayfs CoW layer + Docker container
triple for every branch.  Only files you actually change consume extra disk;
the rest is shared read-only with the main checkout.

## Installation

Download the latest binary from the
[Releases](https://github.com/patricklbell/janus/releases) page:

```sh
# Linux amd64
curl -Lo janus https://github.com/patricklbell/janus/releases/latest/download/janus-linux-amd64
chmod +x janus
sudo mv janus /usr/local/bin/
```

Or install from source with Go (see [Developer instructions](#developer-instructions) below).

## Prerequisites

### Required

| Dependency       | Why                                       | Install                                                      |
|------------------|-------------------------------------------|--------------------------------------------------------------|
| **Docker**       | run isolated containers per worktree      | https://docs.docker.com/engine/install/                      |
| **fuse-overlayfs** | rootless copy-on-write overlay filesystem | `sudo apt-get install fuse-overlayfs` (or `dnf` / `pacman`) |
| **git**          | worktree management                       | `sudo apt-get install git` (or your package manager)         |

### Required groups

After installing, add your user to the required groups (run once, then log out/in):

| Group    | Why                                       | Fix                              |
|----------|-------------------------------------------|----------------------------------|
| `docker` | run containers without sudo               | `sudo usermod -aG docker $USER` |
| `fuse`   | access `/dev/fuse` for rootless overlayfs | `sudo usermod -aG fuse $USER`   |

### Optional

| Dependency                          | Why                                                      |
|-------------------------------------|----------------------------------------------------------|
| **VS Code** + Dev Containers extension | `janus switch` can reopen the editor inside the container |

## One-time setup

Run `janus setup` to verify all prerequisites:

```sh
janus setup   # should show ✓ for all five checks
```

## Quick start

```sh
# In any git repo
janus init

# Create a worktree on a new branch (CoW — no file duplication)
janus new feature-x

# Switch to a branch (auto-creates if needed)
# With VS Code + Dev Containers extension: reopens the editor inside the container
janus switch feature-x

# List worktrees
janus ls

# Open interactive shell inside the container
janus enter feature-x        # or: janus sh feature-x

# Run a command non-interactively
janus exec feature-x -- make test

# Remove worktree (stops container, unmounts overlay, removes git worktree)
janus rm feature-x
```

No `sudo` required for any of the above commands after the one-time setup.

### `janus switch` and VS Code

`janus switch <branch>` starts the worktree and attempts to reopen VS Code
**inside the running container** using the Dev Containers
["Attach to Running Container"](https://code.visualstudio.com/docs/devcontainers/attach-container)
feature.  This requires the
[Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers).

If the worktree contains a `.devcontainer/devcontainer.json` (or
`.devcontainer.json`), `janus switch` will use the
[Dev Container](https://code.visualstudio.com/docs/devcontainers/create-dev-container)
URI scheme instead, allowing VS Code to use the full devcontainer
configuration (extensions, settings, lifecycle hooks, etc.).

No other editors have an equivalent single-command "reopen in container" flow,
so `janus switch` only attempts this for VS Code.  For any other editor, use
`janus enter <branch>` to get a shell inside the container.

### Command aliases

| Full        | Short |
|-------------|-------|
| `new`       | `n`   |
| `switch`    | `sw`  |
| `enter`     | `sh`  |
| `rm/remove` | —     |
| `ls/list`   | —     |

Branch name, full worktree ID, or a unique prefix are all accepted as `<branch>`.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## Developer instructions

Requires [Go](https://go.dev/dl/) 1.24 or later.

```sh
# Install directly from source
go install github.com/patricklbell/janus/cmd/janus@latest

# Or clone and build locally
git clone https://github.com/patricklbell/janus.git
cd janus
go build -o janus ./cmd/janus
```

Run the tests:

```sh
go test ./...
```
