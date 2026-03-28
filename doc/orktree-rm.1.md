---
title: ORKTREE-RM
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-rm - remove an orktree

# SYNOPSIS

**orktree rm** *branch* [**--force**]

# DESCRIPTION

Remove the orktree for the given branch. This unmounts the overlay,
deregisters the git worktree, and deletes all state.

If other orktrees depend on this one as their base (via **--from**),
removal is refused unless **--force** is passed.

If the overlay cannot be unmounted (e.g. a process has its working
directory inside the mount), a lazy unmount is attempted as a fallback.

# OPTIONS

*branch*
: The branch name, orktree ID, or unique prefix of the orktree to remove.

**--force**, **-f**
: Force removal even if the overlay cannot be cleanly unmounted or if
  other orktrees depend on this one.

**--help**, **-h**
: Print usage information.

# EXAMPLES

Remove an orktree:

    orktree rm fix-parser

Force removal when the mount is busy:

    orktree rm fix-parser --force

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
