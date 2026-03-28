---
title: ORKTREE-INIT
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-init - initialize orktree in a directory

# SYNOPSIS

**orktree init** [**--source** *path*]

# DESCRIPTION

Initialize orktree metadata in the given directory (or the current directory).
Creates a **\<repo\>.orktree/** directory next to the source root containing
**state.json** and a **.gitignore** that prevents the directory from being
tracked by any enclosing git repository.

If the directory is a git repository, orktrees will be git-backed
(each orktree gets its own branch and worktree registration).

# OPTIONS

**--source**, **-s** *path*
: Directory to initialize. Defaults to the current directory.

**--help**, **-h**
: Print usage information.

# EXAMPLES

Initialize in the current directory:

    orktree init

Initialize a specific directory:

    orktree init --source /path/to/project

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
