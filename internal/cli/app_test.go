package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/wsfold/internal/testutil"
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

func TestRunCompletionWithoutArgsPrintsSetupHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"completion"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), `eval "$(wsfold completion zsh)"`) {
		t.Fatalf("expected eval setup command in completion help, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `>> ~/.zshrc`) {
		t.Fatalf("expected profile setup command in completion help, got %q", stdout.String())
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

	var stdout bytes.Buffer
	refs, err := resolveCommandRefs(wsfold.NewApp(), "/tmp/workspace", "summon", []string{"summon"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected resolveCommandRefs error: %v", err)
	}
	if !called {
		t.Fatal("expected picker to be called")
	}
	if len(refs) != 0 {
		t.Fatalf("expected no refs on picker cancellation, got %#v", refs)
	}
	if !strings.Contains(stdout.String(), "Selection cancelled") {
		t.Fatalf("expected neutral cancellation message, got %q", stdout.String())
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

func TestResolveCommandRefsDismissWithoutCandidatesIsNoop(t *testing.T) {
	h := testutil.NewHarness(t)
	for _, env := range h.Env() {
		key, value, _ := strings.Cut(env, "=")
		t.Setenv(key, value)
	}
	t.Setenv("WSFOLD_PROJECTS_DIR", "_prj")

	app := wsfold.NewApp()
	var stdout bytes.Buffer
	refs, err := resolveCommandRefs(app, h.Workspace, "dismiss", []string{"dismiss"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("resolveCommandRefs returned error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected no refs for dismiss noop, got %#v", refs)
	}
	if !strings.Contains(stdout.String(), "Nothing to dismiss") {
		t.Fatalf("expected friendly dismiss noop message, got %q", stdout.String())
	}
}

func TestReconcileSelectionTreatsPickerCancellationAsNoop(t *testing.T) {
	original := runPicker
	t.Cleanup(func() { runPicker = original })

	runPicker = func(app *wsfold.App, cwd string, command string, stdout io.Writer, stderr io.Writer) ([]string, error) {
		return nil, errPickerCancelled
	}

	h := testutil.NewHarness(t)
	for _, env := range h.Env() {
		key, value, _ := strings.Cut(env, "=")
		t.Setenv(key, value)
	}
	t.Setenv("WSFOLD_PROJECTS_DIR", "_prj")

	app := wsfold.NewApp()
	var stdout bytes.Buffer
	if err := reconcileSelection(app, h.Workspace, "summon-external", &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("reconcileSelection returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Selection cancelled") {
		t.Fatalf("expected neutral cancellation message, got %q", stdout.String())
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

func TestRunReindexRefreshesCache(t *testing.T) {
	h := testutil.NewHarness(t)
	for _, env := range h.Env() {
		key, value, _ := strings.Cut(env, "=")
		t.Setenv(key, value)
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h.Root, "cache"))

	ghPath := h.WriteExecutable("gh", `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  exit 0
fi
if [ "$1" = "repo" ] && [ "$2" = "list" ] && [ "$3" = "acme" ]; then
  printf '%s\n' '[{"name":"service","nameWithOwner":"acme/service","isPrivate":false,"isArchived":false,"url":"https://github.com/acme/service"}]'
  exit 0
fi
if [ "$1" = "repo" ] && [ "$2" = "list" ] && [ "$3" = "platform-team" ]; then
  printf '%s\n' '[]'
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", filepath.Dir(ghPath))

	var stdout bytes.Buffer
	if err := Run([]string{"reindex"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "refreshed trusted index") {
		t.Fatalf("unexpected reindex output: %q", stdout.String())
	}

	acmeCache, ok, err := loadTrustedCacheForTest("acme")
	if err != nil {
		t.Fatalf("loadTrustedCacheForTest returned error: %v", err)
	}
	if !ok || len(acmeCache.Repos) != 1 || acmeCache.Repos[0].FullName != "acme/service" {
		t.Fatalf("expected acme cache to be refreshed, got %#v", acmeCache)
	}
	if time.Since(acmeCache.FetchedAt) > time.Minute {
		t.Fatalf("expected cache timestamp to be current, got %#v", acmeCache)
	}
}

type trustedCacheForTest struct {
	Org       string    `json:"org"`
	FetchedAt time.Time `json:"fetchedAt"`
	Repos     []struct {
		FullName string `json:"fullName"`
	} `json:"repos"`
}

func loadTrustedCacheForTest(org string) (trustedCacheForTest, bool, error) {
	path, err := trustedRemoteCachePathForTest(org)
	if err != nil {
		return trustedCacheForTest{}, false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return trustedCacheForTest{}, false, nil
		}
		return trustedCacheForTest{}, false, err
	}
	var payload trustedCacheForTest
	return payload, true, json.Unmarshal(raw, &payload)
}

func trustedRemoteCachePathForTest(org string) (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "wsfold", "trusted-github", org+".json"), nil
}
