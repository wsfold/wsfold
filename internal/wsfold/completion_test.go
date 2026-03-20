package wsfold

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/wsfold/internal/testutil"
)

func TestCompleteTrustedAndExternalRepos(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)

	trustedRepo := filepath.Join(h.TrustedRoot, "service")
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	externalRepo := filepath.Join(h.ExternalRoot, "legacy-tool")
	h.InitRepo(externalRepo)
	h.RunGit(externalRepo, "remote", "add", "origin", "https://github.com/other/legacy-tool.git")

	app := NewApp()
	candidates, err := app.Complete(h.Workspace, "summon", "se")
	if err != nil {
		t.Fatalf("Complete summon returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "service" {
		t.Fatalf("unexpected trusted completion candidates: %#v", candidates)
	}
	if !strings.Contains(candidates[0].Description, "acme/service") {
		t.Fatalf("expected origin slug in trusted description, got %#v", candidates[0])
	}

	candidates, err = app.Complete(h.Workspace, "summon-untrusted", "leg")
	if err != nil {
		t.Fatalf("Complete summon-untrusted returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "legacy-tool" {
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

	candidates, err := app.Complete(h.Workspace, "dismiss", "s")
	if err != nil {
		t.Fatalf("Complete dismiss returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "service" {
		t.Fatalf("unexpected dismiss candidates: %#v", candidates)
	}
	if !strings.Contains(candidates[0].Description, "acme/service") {
		t.Fatalf("expected repo ref in dismiss description, got %#v", candidates[0])
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
