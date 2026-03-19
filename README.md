# janus
Zero-cost worktrees for agents

`agentw` is a minimal Linux CLI that creates isolated container workspaces over a
shared source directory using overlayfs + Docker.  Each workspace gets its own
copy-on-write filesystem view and a dedicated Docker container — no git or other
VCS required.

## Quick start

```sh
# Initialise in any directory (no git required)
agentw init [--source <path>] [--image <docker-image>]

# Create a workspace (requires CAP_SYS_ADMIN for the overlay mount)
sudo agentw workspace new [--name <name>]

# List workspaces
agentw workspace ls

# Open interactive shell
agentw workspace enter <id>

# Run a command
agentw workspace exec <id> -- <cmd...>

# Open in editor
agentw workspace open <id> [--editor code|vim|emacs]

# Remove workspace
sudo agentw workspace rm <id>
```

`agentw workspace` can be shortened to `agentw ws`.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.
