---
title: ORKTREE-MOUNT
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-mount - mount an orktree overlay

# SYNOPSIS

**orktree mount** *worktree*

# DESCRIPTION

Mount the fuse-overlayfs overlay for an existing orktree. For stacked
orktrees, ancestor overlays are mounted recursively first.

This is a no-op if the orktree is already mounted.

# OPTIONS

*worktree*
: The orktree to mount, identified by branch name, path, or unique prefix.

# SEE ALSO

**orktree**(1), **orktree-unmount**(1), **orktree-add**(1)
