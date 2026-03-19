# janus
Container worktrees for agents — each branch gets its own isolated workspace
with **no file duplication** (copy-on-write via `fuse-overlayfs`).

`janus` creates a git worktree + fuse-overlayfs CoW layer + Docker container
triple for every branch.  Only files you actually change consume extra disk;
the rest is shared read-only with the main checkout.

## One-time setup

Run `janus setup` to verify all prerequisites.  Two groups are required:

| Group    | Why                                       | Fix (run once, then log out/in) |
|----------|-------------------------------------------|---------------------------------|
| `docker` | run containers without sudo               | `sudo usermod -aG docker $USER` |
| `fuse`   | access `/dev/fuse` for rootless overlayfs | `sudo usermod -aG fuse $USER`   |

`fuse-overlayfs` must also be installed:

```sh
# Debian/Ubuntu
sudo apt-get install fuse-overlayfs

# Fedora/RHEL
sudo dnf install fuse-overlayfs

# Arch
sudo pacman -S fuse-overlayfs
```

After installing and running the two `usermod` commands, log out and back in,
then confirm everything is ready:

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
