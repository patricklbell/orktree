# orktree

Better worktrees for agents and humans — git worktrees with **no file duplication**.

`orktree` pairs a git branch with a fuse-overlayfs CoW layer. Only files you
actually change consume extra disk; the rest is shared read-only with the base
checkout.  Creating a new orktree is **zero-cost**: the existing
checkout (or another orktree's merged view) is used directly as the overlayfs
lowerdir — no files are duplicated.

## Installation

Download the latest binary from the
[Releases](https://github.com/patricklbell/orktree/releases) page:

```sh
# Linux amd64
curl -Lo orktree https://github.com/patricklbell/orktree/releases/latest/download/orktree-linux-amd64
chmod +x orktree
sudo mv orktree /usr/local/bin/
```

Or install from source with Go (see [Developer instructions](#developer-instructions) below).

## Prerequisites

### Required

| Dependency       | Why                                       | Install                                                      |
|------------------|-------------------------------------------|--------------------------------------------------------------|
| **fuse-overlayfs** | rootless copy-on-write overlay filesystem | `sudo apt-get install fuse-overlayfs` (or `dnf` / `pacman`) |
| **git**          | worktree management                       | `sudo apt-get install git` (or your package manager)         |

### Required groups

After installing, add your user to the required groups (run once, then log out/in):

| Group    | Why                                       | Fix                              |
|----------|-------------------------------------------|----------------------------------|
| `fuse`   | access `/dev/fuse` for rootless overlayfs | `sudo usermod -aG fuse $USER`   |


### Check

Run `orktree check` to verify all prerequisites:

```sh
orktree check   # should show ✓ for all checks
```

## Quick start

```sh
# In any git repo
orktree init

# Create an orktree on a new branch (zero-cost — no file duplication)
orktree new feature-x

# Create an orktree branching from an existing orktree (zero-cost stacking)
orktree new feature-x-variant --from feature-x

# Switch to a branch (auto-creates if needed)
orktree switch feature-x

# List orktrees
orktree ls

# Remove orktree (unmounts overlay, removes git worktree)
orktree rm feature-x
```

### Zero-cost orktrees

By default `orktree new` is zero-cost: it registers a new git branch and mounts
a fresh (empty) CoW layer on top of the existing checkout — no files are copied.

```
source root (master checkout)
  └─ feature-a  [upper: your changes only, lowerdir: source root]
       └─ feature-a-spike  [upper: empty, lowerdir: feature-a/merged]
```

Pass `--from <git-ref>` when you need to branch from a specific git commit
that isn’t already present in an existing orktree or the source root; that
path performs a conventional `git worktree add` checkout.

### Command aliases

| Full        | Short |
|-------------|-------|
| `new`       | `n`   |
| `switch`    | `sw`  |
| `rm/remove` | —     |
| `ls/list`   | —     |

Branch name, full orktree ID, or a unique prefix are all accepted as `<branch>`.

## Developer instructions

Requires [Go](https://go.dev/dl/) 1.23 or later.

```sh
# Install directly from source
go install github.com/patricklbell/orktree/cmd/orktree@latest

# Or clone and build locally
git clone https://github.com/patricklbell/orktree.git
cd orktree
go build -o orktree ./cmd/orktree
```

Run the tests:

```sh
go test ./...
```

## Actions
Be a good citizen. If code or documentation does not meet or exceed these standards, fix it!