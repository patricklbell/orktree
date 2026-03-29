# orktree

Better worktrees for agents and humans — git worktrees with **no file duplication**.

`orktree` pairs a git branch with a fuse-overlayfs CoW layer. Only files you
actually change consume extra disk; the rest is shared read-only with the base
checkout.  Creating a new orktree is **zero-cost**: the existing
checkout (or another orktree's merged view) is used directly as the overlayfs
lowerdir — no files are duplicated.

## Installation

### From releases

Download the latest binary from the
[Releases](https://github.com/patricklbell/orktree/releases) page:

```sh
# Linux amd64
curl -Lo orktree https://github.com/patricklbell/orktree/releases/latest/download/orktree-linux-amd64
chmod +x orktree
sudo mv orktree /usr/local/bin/
```

### From source

Requires [Go](https://go.dev/dl/) 1.23 or later.

```sh
go install github.com/patricklbell/orktree/cmd/orktree@latest
```

Or clone and build locally:

```sh
git clone https://github.com/patricklbell/orktree.git
cd orktree
make
make install   # installs to ~/.local by default (PREFIX=~/.local)
```

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

## Quick start

```sh
cd /path/to/your/repo

# Enable the shell wrapper (add to ~/.bashrc or ~/.zshrc)
eval "$(orktree shell-init)"

# Create and enter an orktree on a new branch (zero-cost — no file duplication)
orktree switch feature-x

# Create an orktree branching from an existing orktree (zero-cost stacking)
orktree switch feature-x-variant --from feature-x

# List orktrees
orktree ls

# Return to the source root
orktree switch -

# Remove orktree (unmounts overlay, removes git worktree)
orktree rm feature-x
```

### Zero-cost orktrees

By default `orktree switch` is zero-cost: it registers a new git branch and
mounts a fresh (empty) CoW layer on top of the existing checkout — no files are
copied.

```
source root (master checkout)
  └─ feature-a  [upper: your changes only, lowerdir: source root]
       └─ feature-a-spike  [upper: empty, lowerdir: feature-a/merged]
```

Pass `--from <git-ref>` when you need to branch from a specific git commit
that isn't already present in an existing orktree or the source root; that
path performs a conventional `git worktree add` checkout.

### Command aliases

| Full        | Short |
|-------------|-------|
| `switch`    | `sw`  |
| `rm/remove` | —     |
| `ls/list`   | —     |
| `path`      | `p`   |

Branch name, full orktree ID, or a unique prefix are all accepted as `<branch>`.

## Shell completions

Bash and zsh completions are shipped in the `completions/` directory.
The `make install` target installs them automatically.  To install completions
separately:

```sh
make install-completions   # installs to $PREFIX (~/.local by default)
```

Or source them directly in your shell config:

```sh
# Bash (~/.bashrc)
source /path/to/orktree/completions/orktree.bash

# Zsh (~/.zshrc)
fpath+=(/path/to/orktree/completions)
```

The completion files also include the shell wrapper function (`orktree switch`
automatically `cd`s into the orktree), so you can use them as an alternative to
`eval "$(orktree shell-init)"`.

See the [wiki](https://github.com/patricklbell/orktree/wiki) for advanced
topics.

## Developer instructions

```sh
# Clone and build locally
git clone https://github.com/patricklbell/orktree.git
cd orktree
go build -o orktree ./cmd/orktree
```

Run the tests:

```sh
go test ./...
```