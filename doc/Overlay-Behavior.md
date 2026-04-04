# Overlay Behavior — How Source Changes Propagate

## How the overlay works

Each orktree uses [fuse-overlayfs](https://github.com/containers/fuse-overlayfs)
to layer an empty upper directory on top of an existing checkout (the lower
directory). When you read a file in the orktree:

1. If the file exists in the **upper layer** (you modified or created it in this
   orktree), that version is returned.
2. Otherwise, the file is read from the **lower layer** — the source checkout or
   a parent orktree's merged view.

This is the mechanism that makes orktree creation possibke without copying files.

## What this means in practice

Because unmodified files are read directly from the lower layer, **any change to
the lower layer is immediately visible** in the orktree. There is no snapshot —
the lower layer is live.

### Example

```
source root (lower)                 orktree "feature-x" (upper + lower)
├── main.go         ───────────►    ├── main.go          (read from source)
├── README.md       ───────────►    ├── README.md         (read from source)
└── go.mod          ───────────►    └── go.mod            (read from source)
```

If you edit `main.go` in the source root, that change is **immediately visible**
inside `feature-x` — because `feature-x` hasn't modified `main.go`, so it reads
directly from the source root.

Once you edit `main.go` inside the orktree, the upper layer takes over and the
source root's version is masked:

```
source root (lower)                 orktree "feature-x" (upper + lower)
├── main.go                         ├── main.go          (upper — your version)
├── README.md       ───────────►    ├── README.md         (read from source)
└── go.mod          ───────────►    └── go.mod            (read from source)
```

### Stacked orktrees

The same behavior applies when stacking orktrees with a *commit-ish* argument:

```sh
orktree add ../feature-x
orktree add ../feature-x-v2 feature-x
```

Changes in `feature-x` propagate to `feature-x-v2` for any files that
`feature-x-v2` hasn't modified.

## Recommended workflow: use a dev orktree

Because source-root changes affect all child orktrees, we recommend creating a
**dev orktree** for your daily work:

```sh
# Create a dev orktree — do all your everyday work here
orktree add ../dev

# The source root stays clean and stable
# Only return to it when integrating changes across orktrees
```

This way the source root (which serves as the lower layer for most orktrees)
remains stable. Changes you make in `dev` only affect orktrees that use `dev`
as their base — not every orktree in the repository.

## Comparison with git worktree

| | git worktree | orktree |
|---|---|---|
| **File storage** | Full copy of every file | Only modified files stored |
| **Isolation** | Fully independent | Shares unmodified files with lower layer |
| **Source changes** | No effect on worktrees | Immediately visible in child orktrees |
| **Creation cost** | O(repo size) | O(1) |

This trade-off is fundamental to how orktree achieves instant creation: by
sharing the lower layer live rather than copying it.

## See also

- [Build Tool Integration](Build-Tool-Integration.md) — how path differences affect build caches
- [Container Workflows](Container-Integration.md) — mount considerations for Docker and Podman
