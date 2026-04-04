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

**orktree** creates isolated, copy-on-write workspaces from a git repository.
Each orktree is a git branch paired with a fuse-overlayfs overlay so that
only files you actually modify consume extra disk space.

Creating a new orktree is **zero-cost** by default: the existing checkout
(or another orktree's merged view) is used as the overlayfs lowerdir —
no files are duplicated.

# COMMANDS

**add** *path* [*commit-ish*] [**--** *git-flags*...]
: Create a new orktree at *path*. The branch name is derived from the basename
  of *path*. If *commit-ish* matches an existing orktree, the new orktree is
  stacked on top of it (zero-cost). Otherwise *commit-ish* is treated as a git
  ref to branch from. Arguments after **--** are forwarded to `git worktree add`.

**rm** *worktree*... [**--force**] [**--ignore-untracked**] [**--ignore-tracked**]
: Remove one or more orktrees. Refuses removal if there are dependent
  orktrees. Use **--force** to skip safety checks, or **--ignore-untracked**
  / **--ignore-tracked** for finer-grained control. Commits are preserved in
  git history.

**ls** [**--quiet**]
: List all orktrees with status, size, and path.

**path** *worktree*
: Print the workspace path for an existing orktree. Does not auto-create or
  auto-mount.

**mount** *worktree*
: Mount the overlay for an orktree (and its ancestors if stacked). No-op if
  already mounted.

**unmount** *worktree*
: Unmount the overlay for an orktree.

**move** *worktree* *new-path*
: Unmount, relocate, and remount an orktree at *new-path*.

**doctor**
: Check required and optional prerequisites and diagnose issues.

# ZERO-COST ORKTREES

By default, **orktree add** uses the existing checkout as the overlayfs
lowerdir — no files are copied. Use a *commit-ish* that names an existing
orktree to stack:

    # Branch from source root (zero-cost)
    orktree add ../fix-parser

    # Stack on top of an existing orktree (zero-cost)
    orktree add ../fix-parser-v2 fix-parser

    # Branch from an older commit
    orktree add ../hotfix v1.2.3

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

# SEE ALSO

**orktree-add**(1), **orktree-ls**(1), **orktree-path**(1), **orktree-rm**(1), **git-worktree**(1), **fuse-overlayfs**(1)
