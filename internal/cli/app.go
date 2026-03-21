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

const helpText = `wsfold composes trusted and external repositories around the current workspace.

Usage:
  wsfold init
  wsfold summon [repo-ref]
  wsfold reindex trusted
  wsfold summon-external [repo-ref]
  wsfold dismiss [repo-ref]
  wsfold version
  wsfold completion zsh

Commands:
  init              initialize the current directory as a wsfold workspace
  summon            attach a trusted repository into ./${WSFOLD_PROJECTS_DIR:-_prj}; remote trusted repos are discovered and cloned via gh
  reindex trusted   refresh the trusted GitHub remote cache
  summon-external   add an external repository as a workspace root only
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
		if len(args) != 2 || args[1] != "trusted" {
			return fmt.Errorf("usage: wsfold reindex trusted")
		}
		return app.ReindexTrusted()
	}

	switch args[0] {
	case "summon":
		if len(args) == 1 {
			return reconcileSelection(app, cwd, "summon", stdout, stderr)
		}
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
		if len(args) == 1 {
			return reconcileSelection(app, cwd, "summon-external", stdout, stderr)
		}
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
		return runPicker(app, cwd, command, stdout, stderr)
	case 2:
		return []string{args[1]}, nil
	default:
		return nil, fmt.Errorf("%s accepts zero or one repo ref, got %d arguments", command, len(args)-1)
	}
}

func reconcileSelection(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) error {
	var (
		candidates []wsfold.CompletionCandidate
		err        error
	)
	if command == "summon" {
		state, stateErr := app.TrustedSummonPickerState(cwd)
		if stateErr != nil {
			return stateErr
		}
		candidates = state.Candidates
	} else {
		candidates, err = app.Complete(cwd, command, "")
		if err != nil {
			return err
		}
	}

	selected, err := runPicker(app, cwd, command, stdout, stderr)
	if err != nil {
		return err
	}

	adds, removes := planSelectionChanges(candidates, selected)
	for _, ref := range removes {
		if err := app.Dismiss(cwd, ref); err != nil {
			return err
		}
	}

	for _, ref := range adds {
		switch command {
		case "summon":
			if err := app.Summon(cwd, ref); err != nil {
				return err
			}
		case "summon-external":
			if err := app.SummonUntrusted(cwd, ref); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported reconcile command %q", command)
		}
	}

	return nil
}

func planSelectionChanges(candidates []wsfold.CompletionCandidate, selected []string) (adds []string, removes []string) {
	selectedSet := make(map[string]bool, len(selected))
	for _, ref := range selected {
		selectedSet[ref] = true
	}

	for _, candidate := range candidates {
		switch {
		case candidate.Attached && !selectedSet[candidate.Value]:
			removes = append(removes, candidate.Value)
		case !candidate.Attached && selectedSet[candidate.Value]:
			adds = append(adds, candidate.Value)
		}
	}

	return adds, removes
}
