# Shell Integration

orktree doesn't need shell integration. Orktrees live at normal filesystem
paths, so you can `cd` into them directly:

```sh
cd "$(orktree path myworktree)"
```

For convenience, you can add a tiny helper function.

## Bash / Zsh

Add to your `~/.bashrc` or `~/.zshrc`:

```sh
ork() { cd "$(orktree path "$1")" || return; }
```

Usage:

```sh
ork feature-x
```

## Fish

Add to `~/.config/fish/functions/ork.fish`:

```fish
function ork
  cd (orktree path $argv[1])
end
```

---

This is entirely optional — orktree works fine without it. The `ork` function
is just a shorthand for `cd "$(orktree path ...)"`.
