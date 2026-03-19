# janus
Container worktrees for agents — each branch gets its own isolated workspace.

`janus` creates a Docker container + overlayfs + git worktree triple for every
branch you work on.  Switch between them instantly; each gets its own file
state and running container.

## Quick start

```sh
# In any git repo
janus init [--image <docker-image>]

# Create a worktree on a new branch (requires CAP_SYS_ADMIN for the overlay)
sudo janus new feature-x

# Switch to a branch (auto-creates if needed, reopens editor in-place)
sudo janus switch feature-x

# List worktrees
janus ls

# Open interactive shell
janus enter feature-x        # or: janus sh feature-x

# Run a command non-interactively
janus exec feature-x -- make test

# Open in editor
janus open feature-x [--editor code|vim|emacs]

# Remove worktree (container + overlay + git worktree)
sudo janus rm feature-x
```

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
