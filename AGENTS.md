# AGENTS.md

This file describes information for AI agents working in this repository.

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

## Commands

| Command                        | Description                                              |
|-------------------------------|----------------------------------------------------------|
| `orktree check`                | Check prerequisites; print fix commands for missing ones |
| `orktree init [--source <dir>]` | Initialise orktree in a directory (creates `.orktree/state.json`) |
| `orktree switch <branch> [--from <base>] [--no-git]` | Enter orktree (auto-creates if absent) |
| `orktree switch -`             | Return to the source root                                |
| `orktree ls [--quiet]`         | List all orktrees with status and merged path            |
| `orktree path <branch>`        | Print workspace path (auto-creates if absent)            |
| `orktree rm <branch> [--force]` | Unmount overlay, deregister git worktree, delete state  |
| `orktree shell-init [--shell bash\|zsh]` | Print shell integration snippet (eval in .bashrc/.zshrc) |

`<branch>` accepts: exact branch name, full orktree ID, or a unique prefix.
`orktree new` is a deprecated alias for `orktree switch`.

### Zero-cost orktrees

By default `orktree switch` is zero-cost: the existing checkout (or another
orktree's `merged/` view) is used as the overlayfs lowerdir — no files are
copied.  Pass `--from <git-ref>` to branch from a specific commit that isn't
represented by any existing orktree.

```sh
# Branch from source root (zero-cost)
orktree switch fix-parser

# Stack on top of an existing orktree (zero-cost)
orktree switch fix-parser-v2 --from fix-parser

# Branch from an older commit (conventional checkout)
orktree switch hotfix --from v1.2.3
```

---

## Repository layout

```
cmd/orktree/main.go          ← CLI entry point (commands, flag parsing)
internal/git/git.go          ← git worktree helpers
internal/overlay/overlay.go  ← fuse-overlayfs mount/unmount helpers
internal/state/state.go      ← JSON state read/write + path helpers
internal/state/state_test.go ← state unit tests
doc/*.1.md                   ← man page sources (pandoc markdown)
Makefile                     ← build, test, man page generation, install
```

---

## Build & test

```sh
go build ./...          # compile
go test ./...           # run all tests
go vet ./...            # static analysis
make                    # build binary (output: ./orktree)
make man                # generate man pages (requires pandoc)
make install            # install binary + man pages to $PREFIX (~/.local)
```

The module path is `github.com/patricklbell/orktree` (Go 1.23+).

---

## Code conventions

- Error messages are lowercase, without trailing punctuation, and wrapped with
  `%w` so callers can inspect them.
- All persistent state changes are written atomically (write to a temp file then
  `os.Rename`).
- Path helpers (`GitTreeDir`, `OverlayDirs`, `MountPath`) live on
  `*state.Config` so call sites stay free of path-construction logic.
- Add tests in `internal/state/state_test.go` for any new state behaviour.

---

## Writing style

**Code comments** explain the *why*, not the *what* (the code already shows the *what*). Do not add comments if the why and what are already clear for a senior software engineer. Types that consistently pay off:

- **TODOs**: include actionable context — `// TODO: O(n²) fine for n<100, needs indexing at scale`, not `// TODO: optimize`.
- **References**: link to the paper, spec, or source with a permalink; note any divergence.
- **Load-bearing choices**: if a detail is critical for correctness (e.g. "must be sorted — iteration order matters below"), call it out.
- **"Why not"s**: when you avoid the obvious approach, say why; otherwise someone will "fix" it.
- **Hard-learned lessons**: if a non-obvious fix took 30+ minutes to find, document it.
- **Constant rationale**: explain magic numbers — whether derived, measured, or chosen arbitrarily.

**Commit messages** should capture *why* the codebase changed, not just *what* changed.  For non-trivial commits follow **Problem → Solution → Implications**: what forced the change, the key design decisions, and noteworthy trade-offs or surprises.  Each commit should be one coherent change; use `git add -p` to keep refactoring, features, and fixes separate.


## Actions
- If documentation is out of date, update it.
- If you find code or documentation which does not meet the standards of this document, fix it.