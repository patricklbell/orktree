---
title: ORKTREE-ADD
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-add - create a new orktree

# SYNOPSIS

**orktree add** *path* [*commit-ish*] [**--**] [*git-worktree-add-flags*...]

# DESCRIPTION

Create a new orktree at *path*. Registers a git worktree, creates
overlay directories, and mounts the fuse-overlayfs layer. The
workspace is ready to use immediately when the command returns.

The merged overlay path is the git worktree path — standard git
commands work inside it without any special configuration.

# OPTIONS

*path*
: Where the merged view appears. This also becomes the git worktree
  directory. Can be relative or absolute.

*commit-ish*
: If *commit-ish* matches the name of an existing orktree, the new
  orktree is stacked on top of it (the existing orktree's merged path
  becomes the overlay lowerdir). Otherwise *commit-ish* is treated as
  a git ref and passed through to **git worktree add**.

**--**
: Everything after **--** is forwarded directly to **git worktree add**.
  Common options include **-b** *branch*, **--detach**, **--lock**,
  **--orphan**, and **--force**.

If *commit-ish* is omitted, the branch name defaults to
**basename**(*path*), matching **git worktree add** behaviour.

# EXAMPLES

Create an orktree next to the source root:

    orktree add ../hotfix

Stack on an existing orktree (zero-cost — overlay lower = hotfix's merged path):

    orktree add ../variant hotfix

Create the worktree on a new branch:

    orktree add ../experiment -- -b my-branch

Create a detached HEAD worktree at a specific commit:

    orktree add ../bisect v2.0.0 -- --detach

# SEE ALSO

**orktree**(1), **orktree-rm**(1), **orktree-mount**(1)
