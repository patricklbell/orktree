---
title: ORKTREE
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree - git worktree + fuse-overlayfs manager

# SYNOPSIS

**orktree** *command* [*options*]

# DESCRIPTION

**orktree** creates isolated, copy-on-write workspaces from a git
repository. Each orktree is a git worktree paired with a fuse-overlayfs
overlay so that only files you actually modify consume extra disk space.

The merged overlay path **is** the git worktree path. Standard git
commands, **git worktree list**, **git worktree lock/unlock**, and
**git worktree prune/repair** work alongside orktree.

# COMMANDS

**add** *path* [*commit-ish*] [**\-\-**] [*git-worktree-add-flags*...]
: Create a new orktree at *path*. Registers a git worktree, mounts a
  CoW overlay, and returns with the workspace ready.

**rm** *worktree*... [**\-\-force**] [**\-\-ignore-untracked**] [**\-\-ignore-tracked**]
: Remove one or more orktrees. Refuses removal if there are dependent
  orktrees. Use **\-\-force** to skip safety checks, or **\-\-ignore-untracked**
  / **\-\-ignore-tracked** for finer-grained control. Commits are preserved in
  git history.

**ls** [**\-\-quiet**]
: List all orktrees with branch, status, size, and path.

**path** *worktree*
: Print the merged view path for an existing orktree.

**mount** *worktree*
: Mount the overlay for an existing orktree.

**unmount** *worktree*
: Unmount the overlay for an existing orktree.

**move** *worktree* *new-path*
: Move an orktree to a new path.

**doctor**
: Check required and optional prerequisites and diagnose issues.

**help**
: Show usage information.

# HOW IT WORKS

**orktree add** creates a git worktree then overlays a copy-on-write
fuse-overlayfs layer on top. The existing checkout (or another
orktree's merged view) becomes the overlay lowerdir. Only files you
modify are stored in the upper directory — everything else is shared
read-only from the lower layer.

    myrepo/                          <- source root (git checkout)
    myrepo.orktree/                  <- orktree data
      state.json
      .overlayfs/<id>/upper/         <- CoW changes
      .overlayfs/<id>/work/          <- fuse-overlayfs internal
    ../hotfix/                       <- merged view + git worktree

# STACKING

When the *commit-ish* argument to **orktree add** matches an existing
orktree, the new orktree is stacked on top of it — the existing
orktree's merged path becomes the overlay lowerdir. No files are copied.

    # Create an orktree from the source root
    orktree add ../hotfix

    # Stack on top of hotfix (zero-cost)
    orktree add ../hotfix-v2 hotfix

If *commit-ish* is not an existing orktree, it is treated as a git ref
and passed to **git worktree add**.

# PREREQUISITES

Run `orktree doctor` to check prerequisites.

- **fuse-overlayfs** — rootless copy-on-write overlay filesystem
- **fuse group** — /dev/fuse access
- **git** — version control

Optional:

- **user_allow_other** (/etc/fuse.conf) — allows other users (e.g. the Docker daemon) to access orktree mounts. Required for container bind-mount workflows.

# FILES

**\<repo\>.orktree/**
: Sibling directory created next to the source root. Contains state.json
  and the .overlayfs/ internals.

**\<repo\>.orktree/state.json**
: Per-repository orktree state.

**\<repo\>.orktree/.overlayfs/\<id\>/**
: Internal fuse-overlayfs upper and work directories.

# NOTES

## Overlay propagation

Because each orktree's lower layer is a live reference to the source checkout
(or a parent orktree), changes to unmodified files propagate immediately.
Editing a file in the source root changes it in every child orktree that
hasn't modified that file.

To keep the lower layer stable, create a **dev** orktree for everyday work
and reserve the source root for integration:

    orktree add ../dev

# SEE ALSO

**orktree-add**(1), **orktree-rm**(1), **orktree-ls**(1), **orktree-path**(1), **orktree-mount**(1), **orktree-unmount**(1), **orktree-move**(1), **git-worktree**(1), **fuse-overlayfs**(1)
