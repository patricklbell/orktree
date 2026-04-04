# Restrictions

orktree combines fuse-overlayfs and git worktrees. Each layer has its own
limitations. This page documents them clearly by source so you know what to
expect and where to look for fixes.

---

## orktree / fuse-overlayfs restrictions

These are specific to orktree's use of fuse-overlayfs overlays.

### Linux only

fuse-overlayfs is Linux-only. orktree does not work on macOS, Windows, or BSDs.

### fuse-overlayfs must be installed

The `fuse-overlayfs` binary must be on your `$PATH`. Install it via your
package manager (e.g., `apt install fuse-overlayfs`, `dnf install fuse-overlayfs`).
Run `orktree doctor` to verify.

### /dev/fuse access required

The user running orktree must have access to `/dev/fuse`. On most systems this
means being in the `fuse` group or having appropriate udev rules.

### Inode numbers differ from source

Files seen through the overlay have different inode numbers than the underlying
source files. Tools that rely on inode identity (e.g., hardlink deduplication)
may behave unexpectedly.

### Hardlinks become separate copies

Creating a hardlink in the overlay produces a copy in the upper layer, not a
true hardlink. Subsequent writes to one copy do not affect the other.

### xattr behavior may differ

Extended attribute handling in fuse-overlayfs may not match the behavior of the
underlying native filesystem in all cases.

### Performance overhead

The overlay adds a small overhead per filesystem syscall. For typical
development workloads (compiling, editing, running tests) this is negligible,
but I/O-heavy benchmarks may show measurable differences.

### Containers and FUSE mounts

FUSE mounts are not visible inside containers by default. To bind-mount an
orktree into a Docker or Podman container:

1. Enable `user_allow_other` in `/etc/fuse.conf` so the container runtime can
   access the mount.
2. Bind-mount the orktree path (and the `.orktree/` sibling) into the container
   at their host paths.

See [Container Workflows](Container-Integration.md) for details.

---

## git worktree restrictions

These are git's own limitations, not orktree's. They apply to any use of
`git worktree`, with or without orktree.

### Same branch cannot be checked out in two worktrees

git prevents checking out the same branch in multiple worktrees simultaneously.
Use detached HEADs or different branch names if you need parallel workspaces on
the same commit.

### Submodule support is experimental

git's own documentation notes that submodule support in worktrees is
experimental. Some submodule operations may not work correctly across worktrees.

### Some operations require a clean worktree

Operations like `git worktree move` require the worktree to have a clean
working tree (no uncommitted changes).

### Worktree paths must be unique

Each git worktree must have a unique filesystem path. You cannot create two
worktrees at the same location.

---

## Combined restrictions

These arise from the interaction of overlays and worktrees.

### Stacked orktrees require the parent to stay mounted

When an orktree is stacked on top of another (using the stacking feature), the
parent orktree must remain mounted for the child to function. Unmounting the
parent while the child is in use will cause I/O errors.
