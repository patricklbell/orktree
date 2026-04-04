# Bash completion for orktree.
# Source this file, or install it to /usr/share/bash-completion/completions/orktree.

_orktree_branches() {
  local branches
  branches="$(command orktree ls --quiet 2>/dev/null)" || return
  COMPREPLY+=( $(compgen -W "$branches" -- "$cur") )
}

_orktree() {
  local cur prev words cword
  _init_completion || return

  local commands="add rm remove ls list path p mount unmount umount move mv doctor doc help"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
    return
  fi

  local cmd="${words[1]}"
  case "$cmd" in
    add)
      # add takes a filesystem path then optional commit-ish; default to dir completion
      _filedir -d
      ;;
    rm|remove)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "--force --ignore-untracked --ignore-tracked" -- "$cur") )
      else
        _orktree_branches
      fi
      ;;
    path|p|mount|unmount|umount)
      _orktree_branches
      ;;
    move|mv)
      if [[ $cword -eq 2 ]]; then
        _orktree_branches
      else
        _filedir -d
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
