package wsfold

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestCompleteMarksAlreadyAttachedRepos(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)
	initWorkspace(t, h)

	trustedRepo := filepath.Join(h.TrustedRoot, "service")
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	app := NewApp()
	ghPath := writeFakeGHForCloneTest(t, h, true)
	app.Runner = Runner{Env: []string{
		"GIT_CONFIG_GLOBAL=" + h.GitConfig,
		"PATH=" + prependTestPath(filepath.Dir(ghPath)),
		"WSFOLD_TEST_REMOTES_ROOT=" + h.RemotesRoot,
	}}

	if err := app.Summon(h.Workspace, "service"); err != nil {
		t.Fatalf("Summon returned error: %v", err)
	}

	candidates, err := app.Complete(h.Workspace, "summon", "se")
	if err != nil {
		t.Fatalf("Complete summon returned error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("unexpected trusted completion candidates: %#v", candidates)
	}
	if !candidates[0].Attached {
		t.Fatalf("expected attached flag on already added repo, got %#v", candidates[0])
	}
}

func TestCompleteDismissFromManifest(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)
	initWorkspace(t, h)
	h.CreateGitHubRemote("acme", "service")
	externalRepo := filepath.Join(h.ExternalRoot, "other", "legacy-tool")
	h.InitRepo(externalRepo)
	h.RunGit(externalRepo, "remote", "add", "origin", "https://github.com/other/legacy-tool.git")

	app := NewApp()
	ghPath := writeFakeGHForCloneTest(t, h, true)
	app.Runner = Runner{Env: []string{
		"GIT_CONFIG_GLOBAL=" + h.GitConfig,
		"PATH=" + prependTestPath(filepath.Dir(ghPath)),
		"WSFOLD_TEST_REMOTES_ROOT=" + h.RemotesRoot,
	}}

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
	if !candidates[0].Attached {
		t.Fatalf("expected dismiss candidate to be marked attached, got %#v", candidates[0])
	}
}

func TestTrustedSummonPickerStateMergesCachedRemoteReposAndPrefersLocal(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h.Root, "cache"))

	trustedRepo := filepath.Join(h.TrustedRoot, "math-app")
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	if err := saveTrustedOrgCache(trustedOrgCache{
		Org:       "acme",
		FetchedAt: time.Now().UTC(),
		Repos: []TrustedRemoteRepo{
			{Name: "service", FullName: "acme/service", URL: "https://github.com/acme/service"},
			{Name: "worker", FullName: "acme/worker", URL: "https://github.com/acme/worker"},
		},
	}); err != nil {
		t.Fatalf("saveTrustedOrgCache returned error: %v", err)
	}

	app := NewApp()
	state, err := app.TrustedSummonPickerState(h.Workspace)
	if err != nil {
		t.Fatalf("TrustedSummonPickerState returned error: %v", err)
	}

	if len(state.Candidates) != 2 {
		t.Fatalf("expected merged local+remote candidates, got %#v", state.Candidates)
	}
	if state.Candidates[0].Source != CompletionSourceLocal || state.Candidates[0].Value != "math-app" || state.Candidates[0].Slug != "acme/service" {
		t.Fatalf("expected local candidate to win duplicate slug, got %#v", state.Candidates[0])
	}
	if state.Candidates[1].Source != CompletionSourceRemote || state.Candidates[1].Value != "acme/worker" {
		t.Fatalf("expected remote-only candidate to use canonical slug, got %#v", state.Candidates[1])
	}
}

func TestCompleteSummonRemainsLocalOnly(t *testing.T) {
	h := testutil.NewHarness(t)
	setCompletionEnv(t, h)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h.Root, "cache"))

	if err := saveTrustedOrgCache(trustedOrgCache{
		Org:       "acme",
		FetchedAt: time.Now().UTC(),
		Repos: []TrustedRemoteRepo{
			{Name: "worker", FullName: "acme/worker", URL: "https://github.com/acme/worker"},
		},
	}); err != nil {
		t.Fatalf("saveTrustedOrgCache returned error: %v", err)
	}

	trustedRepo := filepath.Join(h.TrustedRoot, "service")
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	app := NewApp()
	candidates, err := app.Complete(h.Workspace, "summon", "")
	if err != nil {
		t.Fatalf("Complete summon returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Value != "service" {
		t.Fatalf("expected shell completion to stay local-only, got %#v", candidates)
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
