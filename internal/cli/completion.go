package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/openclaw/wsfold/internal/wsfold"
)

const zshCompletionScript = `
autoload -Uz compinit 2>/dev/null || true
if ! typeset -f compdef >/dev/null 2>&1; then
  compinit >/dev/null 2>&1
fi

_wsfold() {
  local context state line
  typeset -A opt_args

  local -a commands
  commands=(
    'summon:attach a trusted repository'
    'summon-untrusted:add an external repository as a workspace root'
    'dismiss:remove a repository from the current composition'
    'version:print build version metadata'
    'completion:print shell completion setup'
  )

  _arguments -C \
    '1:command:->command' \
    '2:repo ref:->repo' \
    '3::arg:->arg'

  case $state in
    command)
      _describe -t commands 'wsfold commands' commands
      return
      ;;
    repo)
      local cmd="${words[2]}"
      local current="${PREFIX}"
      local -a raw
      local -a described
      local line value description

      raw=("${(@f)$(${words[1]} __complete "${cmd}" "${current}" 2>/dev/null)}")
      for line in "${raw[@]}"; do
        value="${line%%$'\t'*}"
        if [[ "$line" == *$'\t'* ]]; then
          description="${line#*$'\t'}"
          described+=("${value}:${description}")
        else
          described+=("${value}")
        fi
      done

      _describe -t repo_refs 'repo refs' described
      return
      ;;
    arg)
      if [[ "${words[2]}" == "completion" ]]; then
        _describe -t shells 'shells' 'zsh:generate zsh completion script'
      fi
      return
      ;;
  esac
}

compdef _wsfold wsfold
`

func writeZshCompletion(w io.Writer) error {
	_, err := io.WriteString(w, zshCompletionScript)
	return err
}

func writeCompletions(cwd string, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("expected a shell name, got %d arguments", len(args)-1)
	}

	switch args[1] {
	case "zsh":
		return writeZshCompletion(stdout)
	default:
		return fmt.Errorf("unsupported shell %q", args[1])
	}
}

func writeDynamicCompletions(cwd string, args []string, stdout io.Writer) error {
	if len(args) != 3 {
		return nil
	}

	app := wsfold.NewApp()
	candidates, err := app.Complete(cwd, args[1], args[2])
	if err != nil {
		return nil
	}

	for _, candidate := range candidates {
		line := candidate.Value
		if strings.TrimSpace(candidate.Description) != "" {
			line += "\t" + candidate.Description
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}

	return nil
}
