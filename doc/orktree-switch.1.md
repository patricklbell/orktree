---
title: ORKTREE-SWITCH
section: 1
header: User Commands
footer: orktree
date: 2025
---

# NAME

orktree-switch - enter an orktree workspace

# SYNOPSIS

**orktree switch** *branch* [**--from** *base*] [**--no-git**]

**orktree switch** **-**

# DESCRIPTION

Enter an orktree for the given branch. If no orktree exists for the branch,
one is created automatically.

When shell integration is active (see **orktree-shell-init**(1)), this command
also changes the working directory to the orktree workspace.

# OPTIONS

*branch*
: The branch name, orktree ID, or unique prefix to switch to.

**-**
: Return to the source root (the original, non-orktree checkout).

**--from**, **-f** *base*
: Base branch, orktree, or git ref to branch from. Only used during creation.
  If *base* is an existing orktree, the new orktree stacks on top (zero-cost).
  If *base* is a git ref, a conventional checkout is performed.

**--no-git**
: Skip git worktree setup. The orktree will not be associated with a git branch.

**--help**, **-h**
: Print usage information.

# EXAMPLES

Create and enter an orktree from the source root (zero-cost):

    orktree switch fix-parser

Stack a new orktree on top of an existing one (zero-cost):

    orktree switch fix-parser-v2 --from fix-parser

Branch from a specific git tag:

    orktree switch hotfix --from v1.2.3

Return to the source root:

    orktree switch -

# SEE ALSO

**orktree**(1), **orktree-rm**(1), **orktree-path**(1)
