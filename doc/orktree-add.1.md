---
title: ORKTREE-ADD
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-add - create a new orktree

# SYNOPSIS

**orktree add** *path* [*commit-ish*] [**--** *git-worktree-add-flags*...]

# DESCRIPTION

Create a new orktree at *path*. The branch name is derived from the basename
of *path* (e.g., `../feature-x` creates branch `feature-x`).

If *commit-ish* matches an existing orktree (by branch name, ID, or path
basename), the new orktree is **stacked** on top of it: the parent orktree's
merged view becomes the overlay lowerdir, and the branch is forked from the
parent's branch. This is zero-cost — no files are copied.

If *commit-ish* does not match an existing orktree, it is treated as a git
ref (branch, tag, or commit). A new branch is created from that ref if the
branch does not already exist.

If *commit-ish* is omitted, the new branch is created from HEAD and the
source root is used as the overlay lowerdir.

Arguments after **--** are forwarded verbatim to `git worktree add`.

The merged path is printed to stdout on success.

# OPTIONS

*path*
: Filesystem path where the orktree workspace will be created. Typically a
  sibling directory of the source root, e.g. `../feature-x`.

*commit-ish*
: Optional. An existing orktree reference (for stacking) or a git ref (branch,
  tag, commit) to base the new branch on.

**--** *git-worktree-add-flags*
: Everything after `--` is forwarded to `git worktree add`.

# EXAMPLES

Create an orktree from the source root (zero-cost):

    orktree add ../fix-parser

Stack a new orktree on top of an existing one (zero-cost):

    orktree add ../fix-parser-v2 fix-parser

Branch from a specific git tag:

    orktree add ../hotfix v1.2.3

Create and cd into the orktree:

    cd "$(orktree add ../feature-x)"

# SEE ALSO

**orktree**(1), **orktree-rm**(1), **orktree-path**(1), **git-worktree**(1)
