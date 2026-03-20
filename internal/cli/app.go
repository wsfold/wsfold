package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/openclaw/wsfold/internal/buildinfo"
	"github.com/openclaw/wsfold/internal/wsfold"
)

const helpText = `wsfold composes trusted and external repositories around the current workspace.

Usage:
  wsfold init
  wsfold summon <repo-ref>
  wsfold summon-untrusted <repo-ref>
  wsfold dismiss <repo-ref>
  wsfold version
  wsfold completion zsh

Commands:
  init              initialize the current directory as a wsfold workspace
  summon            attach a trusted repository into ./${WSFOLD_PROJECTS_DIR:-_prj} and refresh the workspace
  summon-untrusted  add an external repository as a workspace root only
  dismiss           remove a repository from the current composition
  version           print build version metadata
  completion        print shell completion setup
`

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, err := io.WriteString(stdout, helpText)
		return err
	}

	if args[0] == "--version" || args[0] == "version" {
		_, err := fmt.Fprintf(stdout, "wsfold %s (commit %s, built %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if args[0] == "completion" {
		return writeCompletions(cwd, args, stdout)
	}

	if args[0] == "__complete" {
		return writeDynamicCompletions(cwd, args, stdout)
	}

	if len(args) != 2 {
		return fmt.Errorf("expected a command and repo ref, got %d arguments", len(args))
	}

	app := wsfold.NewApp()
	app.Stdout = stdout
	app.Stderr = stderr

	if args[0] == "init" {
		if len(args) != 1 {
			return fmt.Errorf("init does not accept positional arguments")
		}
		return app.Init(cwd)
	}

	switch args[0] {
	case "summon":
		if len(args) != 2 {
			return fmt.Errorf("expected a command and repo ref, got %d arguments", len(args))
		}
		return app.Summon(cwd, args[1])
	case "summon-untrusted":
		if len(args) != 2 {
			return fmt.Errorf("expected a command and repo ref, got %d arguments", len(args))
		}
		return app.SummonUntrusted(cwd, args[1])
	case "dismiss":
		if len(args) != 2 {
			return fmt.Errorf("expected a command and repo ref, got %d arguments", len(args))
		}
		return app.Dismiss(cwd, args[1])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
