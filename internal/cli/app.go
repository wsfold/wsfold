package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/openclaw/wsfold/internal/buildinfo"
	"github.com/openclaw/wsfold/internal/wsfold"
)

const (
	ansiYellow = "\x1b[33m"
	ansiBold   = "\x1b[1m"
	ansiReset  = "\x1b[0m"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		return writeHelp(stdout)
	}

	if args[0] == "--version" || args[0] == "-v" {
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

	app := wsfold.NewApp()
	app.Stdout = stdout
	app.Stderr = stderr

	if args[0] == "init" {
		if len(args) != 1 {
			return fmt.Errorf("init does not accept positional arguments")
		}
		return app.Init(cwd)
	}

	if args[0] == "reindex" {
		if len(args) != 1 {
			return fmt.Errorf("usage: wsfold reindex")
		}
		return app.ReindexTrusted()
	}

	switch args[0] {
	case "summon":
		refs, err := resolveCommandRefs(app, cwd, "summon", args, stdout, stderr)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := app.Summon(cwd, ref); err != nil {
				return err
			}
		}
		return nil
	case "summon-external":
		refs, err := resolveCommandRefs(app, cwd, "summon-external", args, stdout, stderr)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := app.SummonUntrusted(cwd, ref); err != nil {
				return err
			}
		}
		return nil
	case "dismiss":
		refs, err := resolveCommandRefs(app, cwd, "dismiss", args, stdout, stderr)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := app.Dismiss(cwd, ref); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func resolveCommandRefs(app *wsfold.App, cwd string, command string, args []string, stdout io.Writer, stderr io.Writer) ([]string, error) {
	switch len(args) {
	case 1:
		if command == "dismiss" {
			candidates, err := app.Complete(cwd, command, "")
			if err != nil {
				return nil, err
			}
			if len(candidates) == 0 {
				_, _ = fmt.Fprintf(stdout, "%s·%s Nothing to dismiss\n", ansiYellow+ansiBold, ansiReset)
				return nil, nil
			}
		}
		refs, err := runPicker(app, cwd, command, stdout, stderr)
		if err == errPickerCancelled {
			_, _ = fmt.Fprintf(stdout, "%s·%s Selection cancelled\n", ansiYellow+ansiBold, ansiReset)
			return nil, nil
		}
		return refs, err
	case 2:
		return []string{args[1]}, nil
	default:
		return nil, fmt.Errorf("%s accepts zero or one repo ref, got %d arguments", command, len(args)-1)
	}
}
