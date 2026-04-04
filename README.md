# orktree

Instant copy-on-write Git worktrees via overlay filesystem.

## Benchmark: git source repository

| Command             | Time      | Disk Usage |
|---------------------|-----------|------------|
| `git worktree add`  | 0.348 s   | 40 MB      |
| `orktree add`       | 0.038 s   | 56 B       |

Note: `orktree` creation time and initial disk usage are **independent** of repository size.

## Installation

### Package install (recommended)

Download the appropriate package for your distribution and architecture (x86_64/amd64 for non-ARM chips) from the
[releases](https://github.com/patricklbell/orktree/releases) page.

### From Source

[See below](#install-from-source), make sure to install the prerequisites for your distribution.

## Quick start

```sh
cd /path/to/your/repo
orktree add ../feature-x               # create orktree next to repo
orktree add ../variant feature-x        # stack: variant sees feature-x files, changes are isolated
orktree ls                              # list orktrees
orktree rm feature-x variant            # remove orktrees
```

### How it works

`orktree add` registers a git worktree then mounts a copy-on-write fuse-overlayfs layer on top of the existing checkout. Only changed files consume extra disk space — everything else is shared read-only from the lower layer.

### More information

See the [wiki](doc/Home.md) for guides on shell integration, containers, build tools, and more.

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
