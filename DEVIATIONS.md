# Deviations from the CLI Rewrite Plan

The implementation closely follows the original CLI rewrite plan. The core
command surface (`add`/`rm`/`ls`/`path`/`mount`/`unmount`/`move`/`doctor`/`help`)
and the state schema were implemented as specified. The deviations documented
below are minor refinements discovered during implementation.

---

## A. Dual-purpose `commit-ish` argument for stacking

**Planned:** `orktree add <path> [<commit-ish>]` where `<commit-ish>` is a git ref.

**Implemented:** The `<commit-ish>` argument is auto-detected: if it matches an existing orktree (resolved via `FindOrktree`), the new orktree is stacked on top of it with zero-cost copy-on-write. If no match is found, it is treated as a git ref and passed to `git branch`.

**Why:** This keeps the CLI surface simple — stacking and branching from a ref use the same argument position, so `orktree add ../stacked feature-x` works whether `feature-x` is an orktree name or a git tag.

---

## B. Enhanced `rm` interactive confirmation UX

**Planned:** `--force`, `--ignore-untracked`, `--ignore-tracked` flags for controlling removal safety.

**Implemented:** All three flags exist as planned, plus:
- An interactive TTY confirmation prompt when the orktree has uncommitted work
- Smart default (Y/n vs y/N) based on severity — unmerged commits and tracked changes produce a cautious `y/N` default; only minor concerns (untracked/ignored files) produce `Y/n`
- Categorized assessment output listing unmerged commits, tracked changes, untracked files, and ignored file counts
- Dependent orktree blocking — refuses removal even with `--force` if other orktrees depend on this one as an overlay base layer
- Multi-worktree removal with per-worktree error aggregation

**Why:** A destructive command needs confirmation UX, which the plan left unspecified. The dependent-blocking logic is required by the stacking model — removing a base orktree would break its dependents' overlay mounts.

---

## C. `move` cascades to dependent orktrees

**Planned:** `orktree move <worktree> <new-path>` to relocate an orktree.

**Implemented:** `MoveOrktree` unmounts the overlay, calls `git worktree move`, updates `MergedPath` in state, then finds all dependent orktrees (those whose `LowerOrktreeID` matches) and updates their `LowerDir` to the new path. Finally it remounts the overlay at the new location.

**Why:** Without cascading, moving a base orktree would silently break its dependents — their `LowerDir` would point to a non-existent path, and the next mount would fail.

---

## D. `FindOrktree` resolves by multiple strategies

**Planned:** The plan used `<worktree>` arguments without specifying resolution semantics.

**Implemented:** `state.FindOrktree` resolves a reference through a prioritised cascade:
1. Exact ID match
2. Exact branch name match
3. Exact basename of `MergedPath` match
4. Exact absolute `MergedPath` match
5. ID prefix match
6. Branch prefix match
7. Basename of `MergedPath` prefix match

Ambiguous prefix matches (multiple candidates) produce an error.

**Why:** Users refer to orktrees by whichever handle is most convenient — branch name, directory name, or abbreviated ID. The multi-strategy resolution avoids forcing users to remember which identifier type to use.

---

## E. `doctor` categorises checks as required vs optional

**Planned:** `orktree doctor` to diagnose issues.

**Implemented:** The `Prerequisite` struct includes an `Optional` field. Output is split into required checks (fuse-overlayfs, /dev/fuse access, git) and optional checks (`user_allow_other` in `/etc/fuse.conf`). Required failures produce a non-zero exit; optional failures are advisory.

**Why:** `user_allow_other` is only needed for Docker bind-mount workflows. Reporting it as a hard failure would alarm users who don't need container integration.

---

## F. Command aliases beyond the plan

**Planned:** `add`, `rm`, `ls`, `path`, `mount`, `unmount`, `move`, `doctor`, `help`.

**Implemented:** All planned commands exist, plus convenience aliases:
- `remove` → `rm`
- `list` → `ls`
- `p` → `path`
- `umount` → `unmount`
- `mv` → `move`
- `doc` → `doctor`

**Why:** Short aliases reduce typing for power users. `umount` mirrors the standard Unix command name. These are discoverable via the help output.

---

## G. Auto-init on first use

**Planned:** Not specified — the plan implied the workspace already exists.

**Implemented:** `discoverFromCwd()` in `main.go` auto-initializes when no orktree workspace is found but a git repo is detected. It runs `git rev-parse --show-toplevel` and calls `CreateIndex` automatically, printing a message to stderr.

**Why:** Eliminates a setup step. Users can `cd` into any git repo and immediately run `orktree add ../feature-x` without a separate init command.

---

## H. `ls --quiet` prints branch names

**Planned:** `--quiet` flag on `ls`, but output format was not specified.

**Implemented:** Quiet mode prints one branch name per line. Verbose mode (default) shows a `BRANCH / STATUS / SIZE / PATH` table with a total row summarising upper-dir disk usage.

**Why:** Branch-per-line output is easy to pipe into other commands (`orktree ls -q | xargs ...`). The verbose table provides at-a-glance status for interactive use.

---

## I. `overlay.Create` takes 2 arguments, not 3

**Planned:** The plan's overlay API implied `Create(upper, work, merged)` — three directories.

**Implemented:** `overlay.Create(upper, work)` creates only the upper and work directories. The merged directory (mount point) is created separately: either by `git worktree add` (for git-backed orktrees) or by `os.MkdirAll` in `AddOrktree` (for non-git repos or when using `ExtraArgs`).

**Why:** The merged directory's creation is context-dependent — git worktree add creates it as a side effect, so creating it again in `overlay.Create` would be redundant or racy. Separating the concerns keeps each function responsible for exactly what it owns.

---

## J. Non-git repo support via `State.IsGitRepo` instead of `--no-git` flag

**Planned:** Remove the `--no-git` flag.

**Implemented:** The `--no-git` flag was removed as planned. Instead, `State.IsGitRepo` is set automatically during `Init` based on whether the source root is a git repository (`git.IsGitRepo`). When `IsGitRepo` is false, `AddOrktree` skips git worktree registration and uses the source root directly as the overlay lower dir. `CheckRemoveOrktree` treats all dirty files as untracked (no git status classification).

**Why:** Automatic detection is simpler than a flag — users working in a non-git directory get overlay-only behaviour without having to know about `--no-git`.

---

## Summary

The implementation is faithful to the plan. All ten deviations are either
refinements (enhanced UX for `rm`, multi-strategy resolution, auto-init) or
implementation details the plan intentionally left unspecified (overlay API
signatures, quiet-mode output format, alias set).

| Deviation | Category |
|-----------|----------|
| Dual-purpose `commit-ish` for stacking | Refinement — collapses two concepts into one argument |
| Enhanced `rm` confirmation UX | Refinement — essential safety the plan left unspecified |
| `move` cascades to dependents | Refinement — required by the stacking model |
| Multi-strategy `FindOrktree` resolution | Refinement — ergonomic worktree identification |
| `doctor` required/optional split | Refinement — avoids false alarms |
| Command aliases | Addition — convenience, no API surface change |
| Auto-init on first use | Addition — eliminates setup friction |
| `ls --quiet` branch-per-line output | Detail — plan left format unspecified |
| `overlay.Create` takes 2 args | Detail — plan left API signature unspecified |
| `IsGitRepo` auto-detection | Detail — implements the plan's `--no-git` removal differently |
