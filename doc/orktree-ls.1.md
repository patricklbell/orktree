---
title: ORKTREE-LS
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-ls - list orktrees

# SYNOPSIS

**orktree ls** [**\-\-quiet**]

# DESCRIPTION

List all orktrees in the current repository. Output columns:

**BRANCH**
: The git branch associated with the orktree.

**STATUS**
: Mount status (mounted/unmounted).

**SIZE**
: Disk usage of the overlay upper directory — how much space has been
  consumed by files modified within the orktree.

**PATH**
: The merged view path (git worktree location).

The output includes a **total** line showing the combined size of all
orktree upper directories.

For basic worktree information, **git worktree list** also works since
orktree workspaces are standard git worktrees.

# OPTIONS

**\-\-quiet**, **\-q**
: Print one branch name per line with no header or decoration. Suitable for
  scripting and shell completion.

# EXAMPLES

List all orktrees:

    orktree ls

Get branch names only:

    orktree ls --quiet

# SEE ALSO

**orktree**(1), **orktree-add**(1)
