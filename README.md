# orktree

Instant Git worktrees using an overlay filesystem - no file duplication or disk bloat.

## Example: git source repository

| Command             | Time      | Disk Usage |
|---------------------|-----------|------------|
| `git worktree add`  | 0.348 s   | 40 MB      |
| `orktree add`       | 0.038 s   | 56 B       |

Note: `orktree` creation time and initial disk usage are **independent** of repository size.

## Installation

### Package install (recommended)

Download the appropriate package for your distribution and arhictecture (x86_64/amd64 for non-ARM chips) from the
[releases](https://github.com/patricklbell/orktree/releases) page. 

### From Source

[See below](#install-from-source), make sure to install the prerequisites for your distribution.

## Quick start

```sh
cd /path/to/your/repo

# Create an orktree on a new branch and cd into it
cd "$(orktree add ../feature-x)"

# Create an orktree stacked on an existing orktree (you can work in parallel on feature-x-variant)
cd "$(orktree add ../feature-x-variant feature-x)"

# List orktrees
orktree ls

# Remove orktree (safe)
orktree rm feature-x-variant
```

### How it works

By default `orktree add` does the following: register a new git worktree and
mount a fresh (empty) CoW layer on top of an existing checkout.

```
source root (master checkout)
  └─ feature-x  [upper: your changes only, lowerdir: source root]
       └─ feature-x-variant  [upper: empty, lowerdir: feature-x/merged]
```

You can pass a *commit-ish* argument to `orktree add` when you need to branch
from a specific git commit or stack on an existing orktree. If the *commit-ish*
isn't already present in an existing orktree or the source root then orktree
incurs the storage of a conventional `git worktree add`.

### More information

See the [wiki](doc/Home.md) for tips on how to use orktree with existing tools.

## Developer instructions

### Prerequisites

| Dependency         | Why                          | Install (Fedora)                  |
|--------------------|------------------------------|-----------------------------------|
| **git**            | worktrees                    | `sudo dnf install git`            |
| **fuse-overlayfs** | user mode overlay filesystem | `sudo dnf install fuse-overlayfs` |

After installing, ensure your user is in the required groups (run once, then log out/in):

| Group  | Why                                       | Fix                           |
|--------|-------------------------------------------|-------------------------------|
| `fuse` | access `/dev/fuse` for rootless overlayfs | `sudo usermod -aG fuse $USER` |

#### Optional

| Setting              | Why                                                      | Fix                                                      |
|----------------------|----------------------------------------------------------|----------------------------------------------------------|
| `user_allow_other`   | let Docker/Podman access orktrees (see [Container Workflows](doc/Container-Integration.md)) | `echo 'user_allow_other' \| sudo tee -a /etc/fuse.conf` |

Run `orktree doctor` to verify all prerequisites.

### Install from source

Requires [Go](https://go.dev/dl/) 1.23+, [pandoc](https://pandoc.org/) and [make](https://www.gnu.org/software/make/).

```sh
git clone https://github.com/patricklbell/orktree.git
cd orktree
make install       # installs to ~/.local by default (PREFIX=~/.local)
```

### Build and Test
```sh
make build test    # builds to build by default (OUT_DIR=build)
make smoke         # end-to-end smoke tests (requires fuse-overlayfs)
```