package wsfold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/wsfold/internal/testutil"
)

func TestSummonExistingTrustedRepo(t *testing.T) {
	h := testutil.NewHarness(t)
	setEnv(t, h)

	repoPath := filepath.Join(h.TrustedRoot, "acme", "service")
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
		t.Fatalf("mkdir trusted repo parent: %v", err)
	}
	h.InitRepo(repoPath)
	h.RunGit(repoPath, "remote", "add", "origin", "https://github.com/acme/service.git")

	app := NewApp()
	app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

	if err := app.Summon(h.Workspace, "service"); err != nil {
		t.Fatalf("Summon returned error: %v", err)
	}

	link := filepath.Join(h.Workspace, "refs", "service")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if target != repoPath {
		t.Fatalf("unexpected symlink target: %s", target)
	}

	manifestBytes, err := os.ReadFile(manifestPath(h.Workspace))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Count(string(manifestBytes), "repo_ref: acme/service") != 1 {
		t.Fatalf("expected one trusted manifest entry, got:\n%s", string(manifestBytes))
	}

	workspaceBytes, err := os.ReadFile(workspacePath(h.Workspace))
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	if !strings.Contains(string(workspaceBytes), repoPath) {
		t.Fatalf("workspace did not include trusted root:\n%s", string(workspaceBytes))
	}

	before := string(manifestBytes) + string(workspaceBytes)
	if err := app.Summon(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("second Summon returned error: %v", err)
	}
	manifestBytes, _ = os.ReadFile(manifestPath(h.Workspace))
	workspaceBytes, _ = os.ReadFile(workspacePath(h.Workspace))
	after := string(manifestBytes) + string(workspaceBytes)
	if before != after {
		t.Fatal("second summon should be idempotent")
	}
}

func TestSummonMissingTrustedRepoClones(t *testing.T) {
	h := testutil.NewHarness(t)
	setEnv(t, h)
	h.CreateGitHubRemote("acme", "service")

	app := NewApp()
	app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

	if err := app.Summon(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("Summon returned error: %v", err)
	}

	cloned := filepath.Join(h.TrustedRoot, "acme", "service")
	if _, err := os.Stat(filepath.Join(cloned, ".git")); err != nil {
		t.Fatalf("expected clone at %s: %v", cloned, err)
	}
}

func TestSummonUntrustedExistingAndMissingRepo(t *testing.T) {
	t.Run("existing external repo", func(t *testing.T) {
		h := testutil.NewHarness(t)
		setEnv(t, h)

		repoPath := filepath.Join(h.ExternalRoot, "legacy-tool")
		h.InitRepo(repoPath)
		h.RunGit(repoPath, "remote", "add", "origin", "https://github.com/other/legacy-tool.git")

		app := NewApp()
		app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

		if err := app.SummonUntrusted(h.Workspace, "legacy-tool"); err != nil {
			t.Fatalf("SummonUntrusted returned error: %v", err)
		}

		if _, err := os.Lstat(filepath.Join(h.Workspace, "refs", "legacy-tool")); !os.IsNotExist(err) {
			t.Fatalf("expected no symlink under refs, got %v", err)
		}
	})

	t.Run("missing external repo clones", func(t *testing.T) {
		h := testutil.NewHarness(t)
		setEnv(t, h)
		h.CreateGitHubRemote("other", "legacy-tool")

		app := NewApp()
		app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

		if err := app.SummonUntrusted(h.Workspace, "other/legacy-tool"); err != nil {
			t.Fatalf("SummonUntrusted returned error: %v", err)
		}

		cloned := filepath.Join(h.ExternalRoot, "other", "legacy-tool")
		if _, err := os.Stat(filepath.Join(cloned, ".git")); err != nil {
			t.Fatalf("expected external clone: %v", err)
		}
		if strings.HasPrefix(cloned, h.Workspace) {
			t.Fatalf("external clone must stay outside workspace tree: %s", cloned)
		}
	})
}

func TestDismissTrustedAndExternalLifecycle(t *testing.T) {
	h := testutil.NewHarness(t)
	setEnv(t, h)

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

	trustedClone := filepath.Join(h.TrustedRoot, "acme", "service")
	externalClone := filepath.Join(h.ExternalRoot, "other", "legacy-tool")
	trustedLink := filepath.Join(h.Workspace, "refs", "service")

	if err := app.Dismiss(h.Workspace, "service"); err != nil {
		t.Fatalf("Dismiss trusted returned error: %v", err)
	}
	if _, err := os.Lstat(trustedLink); !os.IsNotExist(err) {
		t.Fatalf("expected trusted symlink removal, got %v", err)
	}
	if _, err := os.Stat(trustedClone); err != nil {
		t.Fatalf("trusted checkout should remain on disk: %v", err)
	}

	if err := app.Dismiss(h.Workspace, "other/legacy-tool"); err != nil {
		t.Fatalf("Dismiss external returned error: %v", err)
	}
	if _, err := os.Stat(externalClone); err != nil {
		t.Fatalf("external checkout should remain on disk: %v", err)
	}

	if err := app.Dismiss(h.Workspace, "other/legacy-tool"); err != nil {
		t.Fatalf("repeat dismiss should be idempotent: %v", err)
	}
}

func TestDismissAfterManualSymlinkRemoval(t *testing.T) {
	h := testutil.NewHarness(t)
	setEnv(t, h)
	h.CreateGitHubRemote("acme", "service")

	app := NewApp()
	app.Runner = Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}

	if err := app.Summon(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("Summon returned error: %v", err)
	}

	link := filepath.Join(h.Workspace, "refs", "service")
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link: %v", err)
	}

	if err := app.Dismiss(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("Dismiss returned error: %v", err)
	}
}

func TestEndToEndSmokeScenario(t *testing.T) {
	h := testutil.NewHarness(t)
	setEnv(t, h)
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
	if err := app.Dismiss(h.Workspace, "acme/service"); err != nil {
		t.Fatalf("Dismiss returned error: %v", err)
	}

	trustedClone := filepath.Join(h.TrustedRoot, "acme", "service")
	externalClone := filepath.Join(h.ExternalRoot, "other", "legacy-tool")
	if _, err := os.Stat(trustedClone); err != nil {
		t.Fatalf("trusted clone missing after smoke flow: %v", err)
	}
	if _, err := os.Stat(externalClone); err != nil {
		t.Fatalf("external clone missing after smoke flow: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(h.Workspace, "refs", "service")); !os.IsNotExist(err) {
		t.Fatalf("trusted symlink should be gone after dismiss, got %v", err)
	}

	manifest, err := loadManifest(h.Workspace)
	if err != nil {
		t.Fatalf("loadManifest returned error: %v", err)
	}
	if len(manifest.Trusted) != 0 {
		t.Fatalf("expected no trusted entries after dismiss, got %#v", manifest.Trusted)
	}
	if len(manifest.External) != 1 || manifest.External[0].RepoRef != "other/legacy-tool" {
		t.Fatalf("unexpected final external entries: %#v", manifest.External)
	}

	workspaceBytes, err := os.ReadFile(workspacePath(h.Workspace))
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	if !strings.Contains(string(workspaceBytes), externalClone) {
		t.Fatalf("workspace should still include external root:\n%s", string(workspaceBytes))
	}
	if strings.Contains(string(workspaceBytes), trustedClone) {
		t.Fatalf("workspace should not include dismissed trusted root:\n%s", string(workspaceBytes))
	}
}

func setEnv(t *testing.T, h *testutil.Harness) {
	t.Helper()
	for _, env := range h.Env() {
		key, value, _ := strings.Cut(env, "=")
		t.Setenv(key, value)
	}
}
