# orktree

Git worktrees without file duplication.

[Git worktrees](https://git-scm.com/docs/git-worktree) are a powerful feature which allow you to work in parallel on different
tasks. Unfortunately, when you create a worktree it duplicates every file in the
checkout. `orktree` pairs a git worktree with a fuse-overlayfs CoW layer
so **only your changes** take up disk space.

## Installation

### Package install (recommended)

Download the appropriate package for your distribution and arch from the
[releases](https://github.com/patricklbell/orktree/releases) page.

### From Source

[See below](#install-from-source), make sure to install the prerequisites for your distribution.

## Quick start

```sh
cd /path/to/your/repo

# Create and enter an orktree on a new branch
orktree switch feature-x

# Create an orktree from an existing orktree (you can work in parallel on feature-x-variant)
orktree switch feature-x-variant --from feature-x

# List orktrees
orktree ls

# Return to the source root
orktree switch -

# Remove orktree (safe)
orktree rm feature-x
```

### How it works

By default `orktree switch` does the following: register a new git worktree and
mount a fresh (empty) CoW layer on top of an existing checkout.

```
source root (master checkout)
  └─ feature-x  [upper: your changes only, lowerdir: source root]
       └─ feature-x-variant  [upper: empty, lowerdir: feature-a/merged]
```

You can pass `--from <git-ref>` when you need to branch from a specific git commit.
If `<git-ref>` isn't already present in an existing orktree or the source root then
orktree incurs the storage of a conventional `git worktree add`.

### More information

See the [wiki](doc/Home) for tips on how to use orktree with existing tools.

## Developer instructions

### Prerequisites

| Dependency         | Why                                       | Install                                                     |
|--------------------|-------------------------------------------|-------------------------------------------------------------|
| **fuse-overlayfs** | rootless copy-on-write overlay filesystem | `sudo apt-get install fuse-overlayfs` (or `dnf` / `pacman`) |
| **git**            | worktree management                       | `sudo apt-get install git` (or your package manager)        |

After installing, ensure your user is in the required groups (run once, then log out/in):

| Group  | Why                                       | Fix                           |
|--------|-------------------------------------------|-------------------------------|
| `fuse` | access `/dev/fuse` for rootless overlayfs | `sudo usermod -aG fuse $USER` |


### Install from source

Requires [Go](https://go.dev/dl/) 1.23+, [pandoc](https://pandoc.org/) and [make](https://www.gnu.org/software/make/).

```sh
git clone https://github.com/patricklbell/orktree.git
cd orktree
make install   # installs to ~/.local by default (PREFIX=~/.local)
```

### Build and Test
```sh
make build test    # builds to build by default (OUT_DIR=build)
```