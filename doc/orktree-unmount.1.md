---
title: ORKTREE-UNMOUNT
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-unmount - unmount an orktree overlay

# SYNOPSIS

**orktree unmount** *worktree*

# DESCRIPTION

Unmount the fuse-overlayfs overlay for an existing orktree. Falls back
to a lazy unmount if the filesystem is busy.

# OPTIONS

*worktree*
: The orktree to unmount, identified by branch name, path, or unique prefix.

# SEE ALSO

**orktree**(1), **orktree-mount**(1)
