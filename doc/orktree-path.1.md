---
title: ORKTREE-PATH
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-path - print orktree workspace path

# SYNOPSIS

**orktree path** *branch* [**--from** *base*] [**--no-git**]

# DESCRIPTION

Print the workspace path for the given orktree. If the orktree does not
exist, it is created automatically.  If the orktree is not mounted,
it is mounted.

Use **-** to print the source root path.

Useful in shell integration and container workflows to resolve workspace
and mount paths.

# OPTIONS

*branch*
: The branch name, orktree ID, or unique prefix.

**-**
: Print the source root path.

**--from**, **-f** *base*
: Branch or git ref to base the new orktree on. Only used when auto-creating.

**--no-git**
: Skip git worktree setup when auto-creating.

# EXAMPLES

Print the workspace path:

    orktree path fix-parser

Print the source root:

    orktree path -

Use in a script:

    cd "$(orktree path fix-parser)"

Mount in a container (preserves git functionality):

    srcroot="$(orktree path -)" || exit 1
    docker run --rm -it -v "$srcroot":"$srcroot" -v "$srcroot.orktree":"$srcroot.orktree" -w "$(orktree path fix-parser)" myimage

# SEE ALSO

**orktree**(1), **orktree-switch**(1)
