---
title: ORKTREE-CHECK
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-check - check orktree prerequisites

# SYNOPSIS

**orktree check**

# DESCRIPTION

Check that all prerequisites for orktree are satisfied and print the exact
fix command for anything that is missing.

# PREREQUISITES

**fuse-overlayfs**
: Rootless copy-on-write overlay filesystem. Install with your package manager.

**fuse group**
: /dev/fuse must be accessible. Usually: **sudo usermod -aG fuse $USER**

**git**
: Required for git-backed orktrees.

# SEE ALSO

**orktree**(1)
