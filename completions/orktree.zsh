#compdef orktree
# Zsh completion for orktree.
# Install to /usr/share/zsh/site-functions/_orktree or source directly.

(( $+functions[_orktree_branches] )) || _orktree_branches() {
  local -a branches
  branches=( ${(f)"$(command orktree ls --quiet 2>/dev/null)"} )
  _describe 'worktree' branches
}

_orktree() {
  local -a commands=(
    'add:create a new orktree'
    'rm:remove orktree'
    'remove:alias for rm'
    'ls:list orktrees'
    'list:alias for ls'
    'path:print workspace path'
    'p:alias for path'
    'mount:mount overlay'
    'unmount:unmount overlay'
    'umount:alias for unmount'
    'move:move orktree'
    'mv:alias for move'
    'doctor:diagnose issues'
    'doc:alias for doctor'
    'help:show usage'
  )

  _arguments -C \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe 'orktree command' commands
      ;;
    args)
      case ${words[1]} in
        add)
          _arguments \
            '1:path:_directories' \
            '2:commit-ish:'
          ;;
        rm|remove)
          _arguments \
            '*:worktree:_orktree_branches' \
            '--force[force removal]' \
            '--ignore-untracked[ignore untracked files]' \
            '--ignore-tracked[ignore tracked changes]'
          ;;
        path|p|mount|unmount|umount)
          _arguments \
            '1:worktree:_orktree_branches'
          ;;
        move|mv)
          _arguments \
            '1:worktree:_orktree_branches' \
            '2:new-path:_directories'
          ;;
        ls|list)
          _arguments \
            '--quiet[only print branch names]'
          ;;
        help)
          _describe 'orktree command' commands
          ;;
      esac
      ;;
  esac
}

_orktree "$@"
