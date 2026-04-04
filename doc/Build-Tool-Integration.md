# Build Tool Integration — Working with Cached Builds in Orktrees

## The problem

Build tools cache **absolute paths** — compiler locations, source trees, and
artifact directories. When you create an orktree the workspace lives at a
different path from the source root:

```
/projects/myrepo/        ← source root
/projects/feature-x/     ← orktree workspace (wherever you placed it)
```

If your build was configured in `/projects/myrepo/`, the cached paths inside
`CMakeCache.txt`, `compile_commands.json`, Meson's `build.ninja`, etc. still
reference the old location. This causes **cache misses**, **stale references**,
or **outright build failures** inside the orktree.

---

## General strategies

### 1. Relative paths

Configure your build system to use relative paths where possible. Many tools
support this and it sidesteps the problem entirely.

### 2. Path rewriting

After cd'ing into an orktree, rewrite hardcoded paths in cache and config files
with `sed` or a dedicated script.

### 3. Symlinks

Create a stable symlink that always points to whichever orktree you are
currently working in:

```bash
ln -sfn "$(orktree path feature-x)" /projects/myrepo-active
```

Build tools configured against `/projects/myrepo-active` will follow the
symlink.

### 4. Per-orktree build directories

Keep a separate build directory per orktree. The small disk overhead is usually
negligible compared to the source tree.

---

## CMake

### What breaks

`CMakeCache.txt` stores absolute paths to the source directory
(`CMAKE_HOME_DIRECTORY`) and compiler. When you work in an orktree at
`/projects/feature-x/` but CMake was originally configured in
`/projects/myrepo/`, those paths no longer resolve.

### Solution 1 — Out-of-tree build with path rewriting (recommended)

After cd'ing into an orktree, fix up the cache in place:

```bash
cd "$(orktree path feature-x)"
cd build/
sed -i "s|/original/source/path|$(cd .. && pwd)|g" CMakeCache.txt
cmake .   # reconfigure with corrected paths — usually fast
```

This is the lightest-weight fix: the incremental rebuild picks up right where
you left off.

### Solution 2 — Per-orktree build directories

Keep one build directory per orktree so caches never conflict:

```bash
orktree add ../feature-x
cd "$(orktree path feature-x)"
mkdir -p build && cd build
cmake ..   # fresh configure — no path conflicts
```

### Solution 3 — CMake presets with `${sourceDir}`

CMake presets resolve `${sourceDir}` at configure time, so they automatically
use the correct path in each orktree:

```json
{
  "configurePresets": [{
    "name": "default",
    "binaryDir": "${sourceDir}/build"
  }]
}
```

### Solution 4 — Automated cache rewriting script

For repeated use, wrap the rewriting logic in a script:

```bash
#!/bin/bash
# fix-cmake-cache.sh — run in your build directory after entering an orktree
OLD_SRC=$(grep 'CMAKE_HOME_DIRECTORY:INTERNAL' CMakeCache.txt | cut -d= -f2)
NEW_SRC=$(cd .. && pwd)
if [ "$OLD_SRC" != "$NEW_SRC" ]; then
  sed -i "s|${OLD_SRC}|${NEW_SRC}|g" CMakeCache.txt
  echo "Rewrote CMakeCache.txt: $OLD_SRC → $NEW_SRC"
  cmake .  # reconfigure
fi
```

---

## Other build tools

### Meson

Meson stores absolute paths in `build.ninja`. After entering an orktree, the
simplest fix is:

```bash
meson setup --wipe builddir
```

Or reconfigure in place with `meson configure`.

### Bazel

Bazel sandboxes builds and is generally path-independent. If you hit issues,
set a per-orktree output base:

```bash
bazel --output_base=.bazel-out build //...
```

### Cargo / Rust

Cargo uses relative paths internally — it works out of the box across orktrees.

### Go

The Go module system is path-independent. No special configuration needed.

### Node.js / npm

Usually works, but `node_modules/.cache` may contain stale absolute paths. If
you see odd errors after entering an orktree:

```bash
rm -rf node_modules/.cache
```

---

## Best practice

For clean, reproducible builds, **configure each orktree's build directory
separately**. The disk overhead of a build directory is typically small compared
to the source tree, and you avoid an entire class of stale-path bugs.
