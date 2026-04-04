---
title: ORKTREE-RM
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-rm - remove one or more orktrees

# SYNOPSIS

**orktree rm** *branch*... [**--force**] [**--ignore-untracked**] [**--ignore-tracked**]

# DESCRIPTION

Remove the orktrees for the given branches. For each orktree this unmounts
the overlay, deregisters the git worktree, and deletes all local state.

**Deleting an orktree does not delete any commits.** The branch and its
commits remain in git history and can be switched to again at any time.

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

When multiple branches are specified, each is processed in order.
Errors for individual orktrees are reported inline and processing
continues; a summary error is returned at the end if any removals
failed.

## Dependents

If other orktrees depend on this one as their base layer (stacked via the *commit-ish* argument to **orktree add**), removal is always refused — even with **--force**. The
dependent orktrees must be removed first or re-stacked with a different
base.

# OPTIONS

*branch*...
: One or more branch names, orktree IDs, or unique prefixes of the orktrees to remove.

**--force**, **-f**
: Skip the safety assessment and confirmation prompt, removing the
  orktree immediately. Implies **--ignore-untracked** and
  **--ignore-tracked**. Does **not** override the dependents check —
  orktrees with dependents are always refused.

**--ignore-untracked**
: Do not treat untracked files as a reason to prompt for confirmation.
  Untracked files in the overlay will still be deleted.

**--ignore-tracked**
: Do not treat modified tracked files as a reason to prompt for
  confirmation. Tracked changes in the overlay will still be deleted.

# EXAMPLES

Remove a clean orktree (no prompt):

    orktree rm fix-parser

Remove multiple orktrees at once:

    orktree rm branch-a branch-b branch-c

Force removal, skipping the interactive assessment:

    orktree rm fix-parser --force

Remove even if there are untracked files (but still prompt for tracked changes):

    orktree rm fix-parser --ignore-untracked

# SEE ALSO

**orktree**(1), **orktree-add**(1)
