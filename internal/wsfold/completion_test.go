package wsfold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/wsfold/internal/testutil"
)

func TestCompleteTrustedAndExternalRepos(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)

	trustedRepo := filepath.Join(h.TrustedRoot, "acme", "service")
	if err := os.MkdirAll(filepath.Dir(trustedRepo), 0o755); err != nil {
		t.Fatalf("mkdir trusted repo parent: %v", err)
	}
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	externalRepo := filepath.Join(h.ExternalRoot, "other", "legacy-tool")
	if err := os.MkdirAll(filepath.Dir(externalRepo), 0o755); err != nil {
		t.Fatalf("mkdir external repo parent: %v", err)
	}
	h.InitRepo(externalRepo)
	h.RunGit(externalRepo, "remote", "add", "origin", "https://github.com/other/legacy-tool.git")

	app := NewApp()
	candidates, err := app.Complete(h.Workspace, "summon", "ac")
	if err != nil {
		t.Fatalf("Complete summon returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "acme/service" {
		t.Fatalf("unexpected trusted completion candidates: %#v", candidates)
	}

	candidates, err = app.Complete(h.Workspace, "summon-untrusted", "other")
	if err != nil {
		t.Fatalf("Complete summon-untrusted returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "other/legacy-tool" {
		t.Fatalf("unexpected external completion candidates: %#v", candidates)
	}
}

func TestCompleteDismissFromManifest(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)
	h.CreateGitHubRemote("acme", "service")
	h.CreateGitHubRemote("other", "legacy-tool")

	app := NewApp()
	app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

	if err := app.Summon(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("Summon returned error: %v", err)
	}
	if err := app.SummonUntrusted(h.Workspace, "other/legacy-tool"); err != nil {
		t.Fatalf("SummonUntrusted returned error: %v", err)
	}

	candidates, err := app.Complete(h.Workspace, "dismiss", "a")
	if err != nil {
		t.Fatalf("Complete dismiss returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "acme/service" {
		t.Fatalf("unexpected dismiss candidates: %#v", candidates)
	}
	if !strings.Contains(candidates[0].Description, "trusted") {
		t.Fatalf("expected trusted description, got %#v", candidates[0])
	}
}

func setCompletionEnv(t *testing.T, h *testutil.Harness) {
	t.Helper()
	for _, env := range h.Env() {
		key, value, _ := strings.Cut(env, "=")
		t.Setenv(key, value)
	}
	t.Setenv("WSFOLD_PROJECTS_DIR", "_prj")
}
