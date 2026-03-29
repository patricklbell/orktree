#compdef orktree
# Zsh completion and shell wrapper for orktree.
# Install to /usr/share/zsh/site-functions/_orktree or source directly.

# Shell wrapper: overrides `orktree switch` to cd into the orktree.
if (( ! $+functions[orktree] )); then
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
fi

(( $+functions[_orktree_branches] )) || _orktree_branches() {
  local -a branches
  branches=( ${(f)"$(command orktree ls --quiet 2>/dev/null)"} )
  _describe 'branch' branches
}

_orktree() {
  local -a commands=(
    'switch:enter orktree (auto-creates if absent)'
    'sw:alias for switch'
    'ls:list orktrees with status and size'
    'list:alias for ls'
    'path:print workspace path (auto-creates if absent)'
    'p:alias for path'
    'rm:remove orktree'
    'remove:alias for rm'
    'shell-init:print shell cd-on-switch snippet'
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
        switch|sw)
          _arguments \
            '1:branch:_orktree_branches' \
            '(-f --from)'{-f,--from}'[branch from a specific ref]:ref:_orktree_branches' \
            '--no-git[skip git branch creation]' \
            '--help[show help]'
          ;;
        path|p)
          _arguments \
            '1:branch:_orktree_branches' \
            '(-f --from)'{-f,--from}'[branch from a specific ref]:ref:_orktree_branches' \
            '--no-git[skip git branch creation]' \
            '--help[show help]'
          ;;
        rm|remove)
          _arguments \
            '1:branch:_orktree_branches' \
            '--force[force removal]' \
            '--help[show help]'
          ;;
        ls|list)
          _arguments \
            '--quiet[only print branch names]' \
            '--help[show help]'
          ;;
        shell-init)
          _arguments \
            '--shell[target shell]:shell:(bash zsh)'
          ;;
        help)
          _describe 'orktree command' commands
          ;;
      esac
      ;;
  esac
}

_orktree "$@"
