# AGENTS.md

This file describes information for AI agents working in this repository.

---

## What is orktree?

`orktree` is a CLI tool for creating isolated, copy-on-write orktrees from a git
repository. Each orktree is a git branch paired with a
[fuse-overlayfs](https://github.com/containers/fuse-overlayfs) overlay so that
only files you actually modify consume extra disk space.

```
<repo-parent>/
  <repo-name>/          ← git checkout (work here)
  <repo-name>.orktree/  ← all orktree data (gitignored)
    state.json
    .gitignore           ← contains "*"; prevents tracking by parent git repo
    .overlayfs/          ← overlayfs internals
      <orktree-id>/
        tree/            ← git worktree registration
        upper/           ← CoW writes (overlayfs upperdir)
        work/            ← overlayfs workdir
    <branch-name>/       ← merged view — cd here to work on a branch
```

For a submodule at `/projects/mainrepo/libs/mylib`, orktree data goes to
`/projects/mainrepo/libs/mylib.orktree/` — inside the parent repo's tree, but
excluded from git tracking by the `.gitignore`.

State is stored in `<repo-name>.orktree/state.json` next to the repository root.

---

## Commands

| Command                        | Description                                              |
|-------------------------------|----------------------------------------------------------|
| `orktree switch <branch> [--from <base>] [--no-git]` | Enter orktree (auto-creates if absent; auto-inits) |
| `orktree switch -`             | Return to the source root                                |
| `orktree ls [--quiet]`         | List all orktrees with status and merged path            |
| `orktree path <branch> [--from <base>] [--no-git]` | Print workspace path (auto-creates if absent) |
| `orktree rm <branch> [--force]` | Unmount overlay, deregister git worktree, delete state  |
| `orktree doctor`               | Diagnose issues (check prerequisites)                    |

`<branch>` accepts: exact branch name, full orktree ID, or a unique prefix.
Branch names containing `/` (e.g. `feature/my-branch`) create nested directories
inside the sibling dir (e.g. `myrepo.orktree/feature/my-branch/`).

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
pkg/orktree/orktree.go       ← public Go API (Index, CreateIndex, DiscoverIndex, etc.)
pkg/orktree/orktree_test.go  ← public API tests
cmd/orktree/main.go          ← thin CLI wrapper (flag parsing, output formatting)
completions/orktree.bash     ← bash completion + shell wrapper
completions/orktree.zsh      ← zsh completion + shell wrapper
internal/git/git.go          ← git worktree helpers
internal/overlay/overlay.go  ← fuse-overlayfs mount/unmount helpers
internal/state/state.go      ← JSON state read/write + path helpers
internal/state/state_test.go ← state unit tests
doc/*.1.md                   ← man page sources (pandoc markdown)
test/smoke/                  ← end-to-end smoke tests (bash, require fuse-overlayfs)
Makefile                     ← build, test, man page generation, install
```

### Porcelain vs plumbing

`package orktree` (in `pkg/orktree/`) exposes an `Index` type with methods
for all orktree operations (CreateOrktree, EnsureOrktree, ListOrktrees, RemoveOrktree, FindOrktree).
The CLI in `cmd/orktree/` is a thin wrapper that parses flags, calls Index
methods, and formats output. The library never writes to stdout/stderr or
calls os.Exit.

---

## Build & test

```sh
go build ./...          # compile
go test ./...           # run all tests
go vet ./...            # static analysis
make build              # build binary + man pages (output: build/)
make smoke              # end-to-end smoke tests (requires fuse-overlayfs)
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
  `*state.State` so call sites stay free of path-construction logic.
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