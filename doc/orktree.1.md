---
title: ORKTREE
section: 1
header: User Commands
footer: orktree
date: 2025
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

**switch** *branch* [**--from** *base*] [**--no-git**]
: Enter an orktree, auto-creating it if it doesn't exist. Use **-** to return to the source root.

**ls** [**--quiet**]
: List all orktrees with status, size, and path.

**path** *branch* [**--from** *base*] [**--no-git**]
: Print workspace path (auto-creates if absent).

**rm** *branch* [**--force**]
: Remove an orktree. Refuses removal if there are uncommitted overlay
  changes, unmerged commits, or dependent orktrees. Use **--force** to
  bypass safety checks.

**shell-init** [**--shell** *bash*|*zsh*]
: Print shell integration snippet (eval in .bashrc/.zshrc).

# SHELL INTEGRATION

Add to your shell startup file:

    eval "$(orktree shell-init)"

This enables **cd-on-switch** and **tab completion**. When shell integration
is active, `orktree switch <branch>` changes your working directory to the
orktree workspace.

# ZERO-COST ORKTREES

By default, **orktree switch** uses the existing checkout as the overlayfs
lowerdir — no files are copied. Use **--from** to branch from a specific
base:

    # Branch from source root (zero-cost)
    orktree switch fix-parser

    # Stack on top of an existing orktree (zero-cost)
    orktree switch fix-parser-v2 --from fix-parser

    # Branch from an older commit (conventional checkout)
    orktree switch hotfix --from v1.2.3

# PREREQUISITES

- **fuse-overlayfs** — rootless copy-on-write overlay filesystem
- **fuse group** — /dev/fuse access
- **git** — version control

# FILES

**\<repo\>.orktree/**
: Sibling directory created next to the source root. Contains state.json,
  the .overlayfs/ internals, and one merged-view directory per branch.

**\<repo\>.orktree/state.json**
: Per-repository orktree state.

**\<repo\>.orktree/.overlayfs/\<id\>/**
: Internal fuse-overlayfs upper, work, and git worktree registration directories.

# SEE ALSO

**orktree-switch**(1), **orktree-ls**(1), **orktree-path**(1), **orktree-rm**(1), **git-worktree**(1), **fuse-overlayfs**(1)
