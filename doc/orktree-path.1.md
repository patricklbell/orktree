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

Print the workspace path for an existing orktree. The *worktree* argument
can be a branch name, orktree ID, basename of the merged path, or a unique
prefix of any of these.

This command does **not** auto-create or auto-mount orktrees. If the
orktree does not exist, an error is returned.

# OPTIONS

*worktree*
: The orktree reference (branch name, ID, path basename, or unique prefix).

# EXAMPLES

Print the workspace path:

    orktree path fix-parser

Use in a script:

    cd "$(orktree path fix-parser)"

# SEE ALSO

**orktree**(1), **orktree-add**(1)
