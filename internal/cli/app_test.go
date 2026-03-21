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
	runPicker = func(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) ([]string, error) {
		called = true
		if command != "summon" {
			t.Fatalf("unexpected picker command: %s", command)
		}
		return []string{}, errPickerCancelled
	}

	refs, err := resolveCommandRefs(wsfold.NewApp(), "/tmp/workspace", "summon", []string{"summon"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != errPickerCancelled {
		t.Fatalf("unexpected resolveCommandRefs error: %v", err)
	}
	if !called {
		t.Fatal("expected picker to be called")
	}
	if len(refs) != 0 {
		t.Fatalf("expected no refs on picker cancellation, got %#v", refs)
	}
}

func TestResolveCommandRefsAllowsExplicitRepoRef(t *testing.T) {
	refs, err := resolveCommandRefs(wsfold.NewApp(), "/tmp/workspace", "dismiss", []string{"dismiss", "math-app"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("resolveCommandRefs returned error: %v", err)
	}
	if len(refs) != 1 || refs[0] != "math-app" {
		t.Fatalf("unexpected explicit repo refs: %#v", refs)
	}
}

func TestResolveCommandRefsRejectsExtraArgs(t *testing.T) {
	_, err := resolveCommandRefs(wsfold.NewApp(), "/tmp/workspace", "summon", []string{"summon", "a", "b"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "summon accepts zero or one repo ref") {
		t.Fatalf("unexpected resolveCommandRefs error: %v", err)
	}
}

func TestPlanSelectionChanges(t *testing.T) {
	candidates := []wsfold.CompletionCandidate{
		{Value: "alpha", Attached: true},
		{Value: "beta", Attached: false},
		{Value: "gamma", Attached: true},
	}

	adds, removes := planSelectionChanges(candidates, []string{"alpha", "beta"})
	if len(adds) != 1 || adds[0] != "beta" {
		t.Fatalf("unexpected adds: %#v", adds)
	}
	if len(removes) != 1 || removes[0] != "gamma" {
		t.Fatalf("unexpected removes: %#v", removes)
	}
}
