# Bash completion and shell wrapper for orktree.
# Source this file, or install it to /usr/share/bash-completion/completions/orktree.

# Shell wrapper: overrides `orktree switch` to cd into the orktree.
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

_orktree_branches() {
  local branches
  branches="$(command orktree ls --quiet 2>/dev/null)" || return
  COMPREPLY+=( $(compgen -W "$branches" -- "$cur") )
}

_orktree() {
  local cur prev words cword
  _init_completion || return

  local commands="switch sw ls list path p rm remove doctor doc help"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
    return
  fi

  local cmd="${words[1]}"
  case "$cmd" in
    switch|sw)
      case "$prev" in
        --from|-f)
          _orktree_branches
          return
          ;;
      esac
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "--from --no-git" -- "$cur") )
      else
        _orktree_branches
      fi
      ;;
    path|p)
      case "$prev" in
        --from|-f)
          _orktree_branches
          return
          ;;
      esac
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "--from --no-git" -- "$cur") )
      else
        _orktree_branches
      fi
      ;;
    rm|remove)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "--force" -- "$cur") )
      else
        _orktree_branches
      fi
      ;;
    ls|list)
      COMPREPLY=( $(compgen -W "--quiet" -- "$cur") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
      ;;
  esac
}

complete -F _orktree orktree
