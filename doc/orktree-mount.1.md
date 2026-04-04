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

Mount the fuse-overlayfs overlay for the given orktree. If the orktree is
stacked on another orktree, ancestor overlays are mounted first.

No-op if the overlay is already mounted.

# SEE ALSO

**orktree**(1), **orktree-unmount**(1)
