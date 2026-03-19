# janus
Container worktrees for agents — each branch gets its own isolated workspace.

`janus` creates a Docker container + git worktree pair for every branch you
work on.  Switch between them instantly; each branch gets its own file state
and running container.

## Prerequisites

Run `janus setup` to verify everything is in order.  The only requirement
beyond Docker and git is that your user is in the **`docker` group**:

```sh
sudo usermod -aG docker $USER   # log out and back in to apply
```

## Quick start

```sh
# In any git repo — run janus setup first if needed
janus init

# Create a worktree on a new branch
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

# Remove worktree (stops container, removes git worktree)
janus rm feature-x
```

No `sudo` required for any command.

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
