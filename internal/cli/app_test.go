package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/openclaw/wsfold/internal/wsfold"
)

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output did not contain usage block: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr output: %q", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	err := Run([]string{"nope", "repo"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}

	if !strings.Contains(err.Error(), `unknown command "nope"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "wsfold ") {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr output: %q", stderr.String())
	}
}

func TestRunInitRejectsExtraArgs(t *testing.T) {
	t.Parallel()

	err := Run([]string{"init", "extra"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "init does not accept positional arguments") {
		t.Fatalf("unexpected init error: %v", err)
	}
}

func TestRunCompletionZsh(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"completion", "zsh"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "compdef _wsfold wsfold") {
		t.Fatalf("unexpected completion output: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr output: %q", stderr.String())
	}
}

func TestRunSummonWithoutRepoRefUsesPicker(t *testing.T) {
	original := runPicker
	t.Cleanup(func() { runPicker = original })

	called := false
	runPicker = func(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) (string, error) {
		called = true
		if command != "summon" {
			t.Fatalf("unexpected picker command: %s", command)
		}
		return "picked-repo", nil
	}

	ref, err := resolveCommandRef(wsfold.NewApp(), "/tmp/workspace", "summon", []string{"summon"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("resolveCommandRef returned error: %v", err)
	}
	if !called {
		t.Fatal("expected picker to be called")
	}
	if ref != "picked-repo" {
		t.Fatalf("unexpected picker result: %q", ref)
	}
}

func TestResolveCommandRefAllowsExplicitRepoRef(t *testing.T) {
	ref, err := resolveCommandRef(wsfold.NewApp(), "/tmp/workspace", "dismiss", []string{"dismiss", "math-app"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("resolveCommandRef returned error: %v", err)
	}
	if ref != "math-app" {
		t.Fatalf("unexpected explicit repo ref: %q", ref)
	}
}

func TestResolveCommandRefRejectsExtraArgs(t *testing.T) {
	_, err := resolveCommandRef(wsfold.NewApp(), "/tmp/workspace", "summon", []string{"summon", "a", "b"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "summon accepts zero or one repo ref") {
		t.Fatalf("unexpected resolveCommandRef error: %v", err)
	}
}
