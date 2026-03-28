# AGENTS.md — orktree agent guide

This file describes how AI agents should work within this repository.

---

## What is orktree?

`orktree` is a CLI tool for creating isolated, copy-on-write orktrees from a git
repository. Each orktree is a git branch paired with a
[fuse-overlayfs](https://github.com/containers/fuse-overlayfs) overlay so that
only files you actually modify consume extra disk space.

```
~/.local/share/orktree/<repo-id>/<orktree-id>/
  tree/    ← git worktree registration (.git gitfile; no checkout for zero-cost orktrees)
  upper/   ← copy-on-write writes (overlayfs upperdir; contains .git gitfile override)
  work/    ← overlayfs workdir
  merged/  ← unified view (work here)
```

State is stored in `.orktree/state.json` at the repository root.

---

## Prerequisites

Run `orktree check` to verify all prerequisites:

| Dependency       | Purpose                            | Install                                    |
|------------------|------------------------------------|--------------------------------------------|
| `fuse-overlayfs` | rootless copy-on-write filesystem  | `sudo apt-get install fuse-overlayfs`      |
| `git`            | worktree branch management         | `sudo apt-get install git`                 |
| `fuse` group     | access `/dev/fuse` (one-time)      | `sudo usermod -aG fuse $USER` then log out |

---

## Typical agent workflow

```sh
# 1. One-time: initialise orktree in the repo root
orktree init

# 2. Create an orktree for your task (zero-cost — no file duplication)
orktree new <branch>            # e.g. orktree new fix-parser

# 3. Work inside the merged directory reported by `orktree new`
#    (all writes land in upper/ — the base checkout is unchanged)

# 4. Verify your work, run tests, etc.
cd $(orktree ls | awk '/fix-parser/ {print $NF}')
go test ./...

# 5. Commit from the git worktree registration dir
git -C ~/.local/share/orktree/<repo-id>/<orktree-id>/tree add -A && \
  git -C ~/.local/share/orktree/<repo-id>/<orktree-id>/tree commit -m "..."

# 6. Remove when done
orktree rm <branch>
```

---

## Commands

| Command                        | Description                                              |
|-------------------------------|----------------------------------------------------------|
| `orktree check`                | Check prerequisites; print fix commands for missing ones |
| `orktree init [--source <dir>]` | Initialise orktree in a directory (creates `.orktree/state.json`) |
| `orktree new <branch> [--from <base>] [--no-git]` | Create an orktree on `<branch>` |
| `orktree ls`                   | List all orktrees with status and merged path            |
| `orktree switch <branch>`      | Ensure orktree is mounted; auto-creates if absent        |
| `orktree rm <branch> [--force]` | Unmount overlay, deregister git worktree, delete state  |

`<branch>` accepts: exact branch name, full orktree ID, or a unique prefix.

Aliases: `new` → `n`, `switch` → `sw`, `rm` → `remove`, `ls` → `list`.

### Zero-cost orktrees

By default `orktree new` is zero-cost: the existing checkout (or another
orktree's `merged/` view) is used as the overlayfs lowerdir — no files are
copied.  Pass `--from <git-ref>` to branch from a specific commit that isn't
represented by any existing orktree.

```sh
# Branch from source root (zero-cost)
orktree new fix-parser

# Stack on top of an existing orktree (zero-cost)
orktree new fix-parser-v2 --from fix-parser

# Branch from an older commit (conventional checkout)
orktree new hotfix --from v1.2.3
```

---

## Repository layout

```
cmd/orktree/main.go          ← CLI entry point (commands, flag parsing)
internal/git/git.go          ← git worktree helpers
internal/overlay/overlay.go  ← fuse-overlayfs mount/unmount helpers
internal/state/state.go      ← JSON state read/write + path helpers
internal/state/state_test.go ← state unit tests
```

---

## Build & test

```sh
go build ./...          # compile
go test ./...           # run all tests
go vet ./...            # static analysis
```

The module path is `github.com/patricklbell/orktree` (Go 1.23+). No external
dependencies beyond the standard library.

---

## Code conventions

- Error messages are lowercase, without trailing punctuation, and wrapped with
  `%w` so callers can inspect them.
- All persistent state changes are written atomically (write to a temp file then
  `os.Rename`).
- Path helpers (`GitTreeDir`, `OverlayDirs`, `MountPath`) live on
  `*state.Config` so call sites stay free of path-construction logic.
- Add tests in `internal/state/state_test.go` for any new state behaviour.
