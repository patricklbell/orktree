---
title: ORKTREE-MOVE
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-move - move an orktree to a new path

# SYNOPSIS

**orktree move** *worktree* *new-path*

# DESCRIPTION

Move an orktree to a new location. Unmounts the overlay, moves the git
worktree via **git worktree move**, updates internal state, and
remounts at the new path.

# OPTIONS

*worktree*
: The orktree to move, identified by branch name, path, or unique prefix.

*new-path*
: The new location for the merged view. Can be relative or absolute.

# SEE ALSO

**orktree**(1), **orktree-add**(1)
