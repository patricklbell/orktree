---
title: ORKTREE-PATH
section: 1
header: User Commands
footer: orktree
date: 2026
---

# NAME

orktree-path - print orktree workspace path

# SYNOPSIS

**orktree path** *worktree*

# DESCRIPTION

Print the merged view path for an existing orktree. Exits with an error
if the orktree does not exist.

This command is resolve-only — it does not create or mount anything.
Users can combine it with shell commands for directory switching:

    cd "$(orktree path hotfix)"

# OPTIONS

*worktree*
: The orktree to resolve, identified by branch name, path, or unique prefix.

# EXAMPLES

Print the workspace path:

    orktree path hotfix

Use in a script:

    cd "$(orktree path hotfix)"

# SEE ALSO

**orktree**(1), **orktree-add**(1)
