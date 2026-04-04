# Uninstalling

How to cleanly stop using orktree and remove it from your system.

---

## 1. Remove all orktrees

Unmount and remove every orktree managed by the current repository:

```sh
for w in $(orktree ls --quiet); do orktree rm --force "$w"; done
```

Or remove them individually:

```sh
orktree rm --force feature-x
orktree rm --force hotfix-123
```

## 2. Clean up git worktrees

After removing orktrees, prune any stale git worktree references:

```sh
git worktree prune
```

## 3. Delete orktree data

Remove the `.orktree/` sibling directory that stores state and overlay
internals:

```sh
rm -rf "$(git rev-parse --show-toplevel).orktree"
```

## 4. Uninstall the binary

If you installed with `make install`:

```sh
make uninstall
```

Or remove manually:

```sh
rm ~/.local/bin/orktree
```

If man pages were installed, remove them too:

```sh
rm ~/.local/share/man/man1/orktree*.1
```

If shell completions were installed, remove them:

```sh
rm -f ~/.local/share/bash-completion/completions/orktree
rm -f ~/.local/share/zsh/site-functions/_orktree
```

---

## What orktree does NOT touch

orktree does not modify your git repository's history, branches, or
configuration. It only:

- Adds git worktrees (which git manages via `.git/worktrees/`)
- Stores overlay state in the sibling `.orktree/` directory

After removing orktrees and deleting the `.orktree/` directory, your
repository is exactly as it was before you started using orktree.

## If something goes wrong

If orktree exits unexpectedly or a mount becomes stale:

1. **Unmount leftover FUSE mounts** — check with `mount | grep fuse-overlayfs`
   and unmount with `fusermount -u <path>`.
2. **Prune git worktrees** — `git worktree prune` cleans up any dangling
   worktree references that point to paths that no longer exist.
3. **Delete the `.orktree/` directory** — this is always safe as a last resort.
   You'll lose any uncommitted changes stored only in overlay upper layers, but
   your git history and branches are unaffected.
