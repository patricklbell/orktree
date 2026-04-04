# Shell Integration

## Using orktree from the command line

With the `switch` command removed, orktree uses explicit `add` and `path`
commands. To change your working directory to an orktree:

    cd "$(orktree path feature-x)"

To create a new orktree and cd into it in one step:

    cd "$(orktree add ../feature-x)"

---

## Completions

Source the completion script for tab completion of orktree commands and
worktree names:

```bash
# Bash (~/.bashrc)
source /path/to/completions/orktree.bash

# Zsh — copy to a directory in $fpath:
cp completions/orktree.zsh /usr/local/share/zsh/site-functions/_orktree
```

If you installed via `make install` or a package, completions are already in place.

---

## Shell aliases (optional)

For convenience you can define shell aliases or functions:

### Bash / Zsh

```bash
# ~/.bashrc or ~/.zshrc
ork() {
  if [[ "$1" == "add" ]]; then
    shift
    cd "$(command orktree add "$@")" || return $?
  else
    command orktree "$@"
  fi
}
```

### Fish

```fish
# ~/.config/fish/functions/ork.fish
function ork
  if test "$argv[1]" = "add"
    set -l _path (command orktree add $argv[2..])
    and cd $_path
  else
    command orktree $argv
  end
end
```

---

## Troubleshooting

### `orktree: command not found`

The binary is not on your `$PATH`. Verify with:

```bash
which orktree        # or: command -v orktree
```

If you installed to `~/.local/bin`, make sure `export PATH="$HOME/.local/bin:$PATH"` is in your rc file.

### Completions not working

- **Bash**: Make sure `bash-completion` is installed (`apt install bash-completion` on Debian/Ubuntu).
- **Zsh**: Ensure `compinit` is called in your `~/.zshrc` and the completion file is in a directory listed in `$fpath`.
