---
title: ORKTREE-LS
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-ls - list orktrees

# SYNOPSIS

**orktree ls** [**--quiet**]

# DESCRIPTION

List all orktrees in the current repository with their branch name, ID,
mount status, and workspace path.

# OPTIONS

**--quiet**, **-q**
: Print one branch name per line with no header or decoration. Suitable for
  scripting and shell completion.

**--help**, **-h**
: Print usage information.

# EXAMPLES

List all orktrees:

    orktree ls

Get branch names only:

    orktree ls --quiet

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
