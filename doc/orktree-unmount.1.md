---
title: ORKTREE-UNMOUNT
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-unmount - unmount an orktree overlay

# SYNOPSIS

**orktree unmount** *worktree*

# DESCRIPTION

Unmount the fuse-overlayfs overlay for the given orktree. No-op if not
currently mounted.

**Warning:** Unmounting while processes are using the workspace may cause
errors. The overlay may fall back to lazy unmount if the mount point is busy.

# SEE ALSO

**orktree**(1), **orktree-mount**(1)
