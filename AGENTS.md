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
  <repo-name>.orktree/  ← orktree data
    state.json
    .overlayfs/<orktree-id>/upper|work
  <path>/               ← merged view + git worktree (wherever user specifies)
```

State is stored in `<repo-name>.orktree/state.json` next to the repository root.

---

## Repository layout

```
pkg/orktree/orktree.go       ← public Go API (Index, CreateIndex, DiscoverIndex, etc.)
pkg/orktree/orktree_test.go  ← public API tests
cmd/orktree/main.go          ← thin CLI wrapper (flag parsing, output formatting)
internal/git/git.go          ← git worktree helpers
internal/overlay/overlay.go  ← fuse-overlayfs mount/unmount helpers
internal/state/state.go      ← JSON state read/write + path helpers
internal/state/state_test.go ← state unit tests
doc/*.1.md                   ← man page sources (pandoc markdown)
test/smoke/                  ← end-to-end smoke tests (bash, require fuse-overlayfs)
.opencode/                   ← OpenCode agents + custom tools for sandboxed orchestration
Makefile                     ← build, test, man page generation, install
```

### Porcelain vs plumbing

`package orktree` (in `pkg/orktree/`) exposes an `Index` type with methods for
all orktree operations. The CLI in `cmd/orktree/` is a thin wrapper: parses
flags, calls `Index` methods, formats output. **The library never writes to
stdout/stderr or calls `os.Exit`.**

---

## Build & test

```sh
go build ./...          # compile
go test ./...           # run all unit tests
go vet ./...            # static analysis
make build              # build binary + man pages (output: build/)
make smoke              # end-to-end smoke tests (requires fuse-overlayfs)
make install            # install binary + man pages to $PREFIX (~/.local)
```

The module path is `github.com/patricklbell/orktree` (Go 1.23+).
There are **no external dependencies** — standard library only.

### Running a single test

```sh
# Run one test function by name (any package)
go test ./... -run TestFunctionName

# Run tests in a specific package
go test ./internal/state/ -run TestFindOrktreeByBranch
go test ./pkg/orktree/    -run TestCreateIndex_idempotent

# Run a specific sub-test (table-driven tests use t.Run)
go test ./pkg/orktree/ -run TestRemoveCheck_IsClean/only_ignored_dirty_is_clean

# Run a single smoke test script directly
bash test/smoke/test_add_basic.sh
```

---

## Code conventions

### Errors

- Messages are **lowercase**, no trailing punctuation:
  `fmt.Errorf("reading state: %w", err)` not `"Reading state failed."`.
- Always wrap with `%w` so callers can use `errors.Is`/`errors.As`.
- Use `errors.New` (not `fmt.Errorf`) when there is no underlying error to wrap.
- Exit codes from `exec.Command` are inspected via `*exec.ExitError` to distinguish
  expected non-zero exits (e.g. "ref does not exist") from real failures.
- Best-effort / cleanup calls that can be safely ignored are annotated
  `//nolint:errcheck`, never silently swallowed without a comment.

### State and paths

- All persistent state changes are written atomically: write to a temp file
  then `os.Rename`. Permissions on the state file are `0o600`.
- Path helpers (`GitTreeDir`, `OverlayDirs`, `MountPath`, `SiblingDir`) live on
  `*state.State`/`*state.Config`. **Never build orktree paths inline at call sites.**
- Add tests in `internal/state/state_test.go` for any new state behaviour.

### Naming

- Exported: `PascalCase` — types (`Index`, `OrktreeMetadata`), methods, constants (`StateFile`).
- Unexported: `camelCase` — helpers, variables, unexported constants.
- Test functions: `Test<Thing>` or `Test<Thing>_<scenario>`, e.g.
  `TestCreateIndex_idempotent`, `TestLoadIndex_failsIfNotInitialized`.
- JSON struct tags: `snake_case`; use `omitempty` on optional fields.

### Imports

Group stdlib first, then a blank line, then local module imports:

```go
import (
    "fmt"
    "os"

    "github.com/patricklbell/orktree/internal/git"
    "github.com/patricklbell/orktree/internal/state"
)
```

### Testing

- Test files use the **external package** convention (`package orktree_test`,
  `package state_test`) — black-box testing against the public API only.
- Prefer **table-driven tests** with `t.Run`. Sub-test names use `snake_case`,
  e.g. `"only_ignored_dirty_is_clean"`.
- Section dividers in large files:
  ```go
  // ---------------------------------------------------------------------------
  // Section name
  // ---------------------------------------------------------------------------
  ```

---

## Writing style

**Code comments** explain the *why*, not the *what*. Omit when obvious to a senior engineer. Always include:

- **TODOs** with actionable context: `// TODO: O(n²) fine for n<100, needs indexing at scale`.
- **Load-bearing choices**: correctness-critical details that look removable but aren't.
- **"Why not"s**: explain rejected obvious approaches so they aren't "fixed" later.
- **Constant rationale**: magic numbers should state whether derived, measured, or arbitrary.

**Commit messages** follow **Problem → Solution → Implications**: what forced the
change, the key design decisions, and notable trade-offs. Use `git add -p` to keep
refactoring, features, and fixes in separate commits.

---

## Actions

- If documentation is out of date, update it.
- If you find code or documentation which does not meet the standards of this document, fix it.
- After every non-trivial change verify: `go build ./... && go test ./... && go vet ./...` all pass.
