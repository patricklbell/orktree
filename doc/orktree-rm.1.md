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

Before removing, **orktree rm** performs several safety checks and
refuses removal when any of the following are true:

- The overlay has **uncommitted changes** (files written to the upper
  directory that have not been committed).
- The branch has **unmerged commits** that do not appear in any other
  branch.
- Other orktrees **depend on this one** as their base (stacked via
  **--from**).

Pass **--force** to bypass all safety checks.

If the overlay cannot be unmounted (e.g. a process has its working
directory inside the mount), a lazy unmount is attempted as a fallback.

# OPTIONS

*branch*
: The branch name, orktree ID, or unique prefix of the orktree to remove.

**--force**, **-f**
: Bypass all safety checks and force removal. This skips the uncommitted
  changes, unmerged commits, and dependent orktrees checks, and also
  forces unmount when the overlay is busy.

**--help**, **-h**
: Print usage information.

# EXAMPLES

Remove an orktree:

    orktree rm fix-parser

Force removal when the mount is busy:

    orktree rm fix-parser --force

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
