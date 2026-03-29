# Shell Integration — cd on switch

## Why you need a shell wrapper

`orktree switch` prints the target workspace path to stdout. However, a child
process cannot change the calling shell's working directory — the kernel
enforces per-process `cwd`. To actually land in the orktree after switching you
need a thin shell wrapper (or alias) that:

1. Calls `command orktree path <branch> ...` to resolve the workspace path.
2. `cd`s into that path inside the current shell session.

Everything else (`orktree ls`, `orktree rm`, …) is forwarded to the real binary
unchanged.

---

## Quick setup

The fastest way to enable cd-on-switch is to add one line to your shell rc file:

```bash
eval "$(orktree shell-init)"   # works for bash and zsh
```

Alternatively, install the completion scripts shipped with orktree
(`completions/orktree.bash` or `completions/orktree.zsh`) — they already include
the wrapper function alongside tab completions.

---

## Bash

The `completions/orktree.bash` file bundles the wrapper and tab completion
together. Source it once in `~/.bashrc`:

```bash
source /path/to/completions/orktree.bash
```

Or, if you only want the cd wrapper without completions:

```bash
# ~/.bashrc
orktree() {
  case "$1" in
    switch|sw)
      local _orktree_path
      _orktree_path="$(command orktree path "${@:2}")" && cd "$_orktree_path" || return $?
      ;;
    *)
      command orktree "$@"
      ;;
  esac
}
```

Reload with `source ~/.bashrc` or open a new terminal.

---

## Zsh

The `completions/orktree.zsh` file bundles the wrapper and tab completion.
Install it as a completion function:

```zsh
# Copy to a directory in your $fpath, e.g.:
cp completions/orktree.zsh /usr/local/share/zsh/site-functions/_orktree
# then restart zsh or run: autoload -Uz compinit && compinit
```

Or add just the wrapper to `~/.zshrc`:

```zsh
# ~/.zshrc
orktree() {
  case "$1" in
    switch|sw)
      local _orktree_path
      _orktree_path="$(command orktree path "${@:2}")" && cd "$_orktree_path" || return $?
      ;;
    *)
      command orktree "$@"
      ;;
  esac
}
```

---

## Fish

Fish uses autoloaded function files. Save the following to
`~/.config/fish/functions/orktree.fish`:

```fish
function orktree
  switch $argv[1]
    case switch sw
      set -l _orktree_path (command orktree path $argv[2..])
      and cd $_orktree_path
    case '*'
      command orktree $argv
  end
end
```

Fish loads the function automatically on next shell start — no `source` needed.

---

## POSIX sh

For `/bin/sh`, `dash`, or other strictly POSIX shells, `local` is not available.
Use a subshell-safe variant:

```sh
# ~/.profile or sourced rc file
orktree() {
  case "$1" in
    switch|sw)
      shift
      _orktree_path="$(command orktree path "$@")" && cd "$_orktree_path" || return $?
      unset _orktree_path
      ;;
    *)
      command orktree "$@"
      ;;
  esac
}
```

> **Note:** POSIX sh does not support `"${@:2}"` (substring parameter
> expansion). The wrapper above uses `shift` to discard the first argument
> before forwarding `"$@"`.

---

## How it works

```
┌─────────────────────────────┐
│ your shell (bash, zsh, …)   │
│                             │
│  orktree switch feature-x   │  ← you type this
│        │                    │
│        ▼                    │
│  wrapper function           │
│    path=$(command orktree   │  ← runs the real binary
│           path feature-x)   │
│    cd "$path"               │  ← changes cwd in *this* shell
└─────────────────────────────┘
```

The wrapper intercepts `switch` (and its alias `sw`), delegates to
`orktree path` to resolve (and auto-create) the orktree, then `cd`s into the
returned path. All other subcommands pass through to the binary directly.

`command orktree` ensures the real binary is invoked rather than the wrapper
recursing into itself.

---

## Troubleshooting

### `orktree: command not found`

The binary is not on your `$PATH`. Verify with:

```bash
which orktree        # or: command -v orktree
```

If you installed to `~/.local/bin`, make sure `export PATH="$HOME/.local/bin:$PATH"` appears **before** the wrapper/completion source line.

### `cd` does nothing after `orktree switch`

You are calling the raw binary instead of the wrapper function. Check:

```bash
type orktree
# Should say "orktree is a function", NOT "orktree is /usr/local/bin/orktree"
```

If it shows a file path, the wrapper is not loaded. Re-source your rc file or
open a new terminal.

### Wrapper not loaded in non-interactive scripts

Shell rc files (`~/.bashrc`, `~/.zshrc`) are only sourced for interactive
sessions. If you need cd-on-switch in a script, source the wrapper explicitly at
the top of the script:

```bash
source /path/to/completions/orktree.bash
```

### Completions not working

- **Bash**: Make sure `bash-completion` is installed (`apt install bash-completion` on Debian/Ubuntu).
- **Zsh**: Ensure `compinit` is called in your `~/.zshrc` and the completion file is in a directory listed in `$fpath`.
