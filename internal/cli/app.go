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
  wsfold summon <repo-ref>
  wsfold summon-untrusted <repo-ref>
  wsfold dismiss <repo-ref>
  wsfold version

Commands:
  summon            attach a trusted repository into the configured projects directory and refresh the workspace
  summon-untrusted  add an external repository as a workspace root only
  dismiss           remove a repository from the current composition
  version           print build version metadata
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

	if len(args) != 2 {
		return fmt.Errorf("expected a command and repo ref, got %d arguments", len(args))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	app := wsfold.NewApp()
	app.Stdout = stdout
	app.Stderr = stderr

	switch args[0] {
	case "summon":
		return app.Summon(cwd, args[1])
	case "summon-untrusted":
		return app.SummonUntrusted(cwd, args[1])
	case "dismiss":
		return app.Dismiss(cwd, args[1])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
