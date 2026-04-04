---
title: ORKTREE-MOVE
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-move - relocate an orktree

# SYNOPSIS

**orktree move** *worktree* *new-path*

# DESCRIPTION

Relocate the orktree identified by *worktree* to *new-path*. This
unmounts the overlay, moves the git worktree registration, updates
orktree state, and remounts the overlay at the new location.

# SEE ALSO

**orktree**(1), **orktree-add**(1)
