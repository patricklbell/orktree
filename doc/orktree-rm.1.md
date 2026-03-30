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

If the orktree is clean (no changed files, no unmerged commits), it is
removed immediately without prompting.

When the orktree has unsaved work, **orktree rm** prints a categorized
assessment of what would be lost:

**Commits only on this branch:**
: Commits reachable only from this branch (not merged into any other
  branch). Up to 10 are listed; the remainder is shown as a count.

**Modified tracked files:**
: Files that differ from the base. Up to 10 are listed.

**Untracked files:**
: New files not covered by *.gitignore*. Up to 10 are listed.

**Ignored files:**
: Gitignored files (build artifacts, caches). Only a count is shown.

In a terminal, the user is prompted for confirmation. The default
depends on the severity of the changes:

- **[y/N]** (default No) when there are unmerged commits or modified
  tracked files.
- **[Y/n]** (default Yes) when only untracked or ignored files remain.

In a non-interactive environment (no TTY), the assessment is printed
followed by a message to pass **--force** to remove without
confirmation, and the command exits with an error.

## Dependents

If other orktrees depend on this one as their base layer (stacked via
**--from**), removal is always refused — even with **--force**. The
dependent orktrees must be removed first or re-stacked with a different
base.

# OPTIONS

*branch*
: The branch name, orktree ID, or unique prefix of the orktree to remove.

**--force**, **-f**
: Skip the safety assessment and confirmation prompt, removing the
  orktree immediately. Does **not** override the dependents check —
  orktrees with dependents are always refused.

# EXAMPLES

Remove a clean orktree (no prompt):

    orktree rm fix-parser

Force removal, skipping the interactive assessment:

    orktree rm fix-parser --force

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
