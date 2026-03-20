package wsfold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/wsfold/internal/testutil"
)

func TestDiscoverRepositoriesFindsNestedAndFlatLayouts(t *testing.T) {
	h := testutil.NewHarness(t)

	trustedRepo := filepath.Join(h.TrustedRoot, "acme", "service")
	if err := os.MkdirAll(filepath.Dir(trustedRepo), 0o755); err != nil {
		t.Fatalf("mkdir trusted repo parent: %v", err)
	}
	h.InitRepo(trustedRepo)
	h.RunGit(trustedRepo, "remote", "add", "origin", "https://github.com/acme/service.git")

	externalRepo := filepath.Join(h.ExternalRoot, "legacy-tool")
	h.InitRepo(externalRepo)

	cfg := Config{
		TrustedDir:        h.TrustedRoot,
		ExternalDir:       h.ExternalRoot,
		TrustedGitHubOrgs: []string{"acme"},
	}

	index, err := DiscoverRepositories(cfg, Runner{})
	if err != nil {
		t.Fatalf("DiscoverRepositories returned error: %v", err)
	}

	if len(index.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(index.Repos))
	}

	repo, err := index.Resolve("acme/service", TrustClassTrusted)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if repo.CheckoutPath != trustedRepo {
		t.Fatalf("unexpected trusted repo path: %s", repo.CheckoutPath)
	}

	repo, err = index.Resolve("legacy-tool", TrustClassExternal)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if repo.CheckoutPath != externalRepo {
		t.Fatalf("unexpected external repo path: %s", repo.CheckoutPath)
	}
}

func TestDiscoverRepositoriesSkipsHiddenDirectories(t *testing.T) {
	t.Parallel()

	h := testutil.NewHarness(t)

	visibleRepo := filepath.Join(h.TrustedRoot, "acme", "service")
	if err := os.MkdirAll(filepath.Dir(visibleRepo), 0o755); err != nil {
		t.Fatalf("mkdir visible repo parent: %v", err)
	}
	h.InitRepo(visibleRepo)

	hiddenRepo := filepath.Join(h.TrustedRoot, ".cache", "ignored")
	if err := os.MkdirAll(filepath.Dir(hiddenRepo), 0o755); err != nil {
		t.Fatalf("mkdir hidden repo parent: %v", err)
	}
	h.InitRepo(hiddenRepo)

	cfg := Config{
		TrustedDir:  h.TrustedRoot,
		ExternalDir: h.ExternalRoot,
	}

	index, err := DiscoverRepositories(cfg, Runner{})
	if err != nil {
		t.Fatalf("DiscoverRepositories returned error: %v", err)
	}

	if len(index.Repos) != 1 {
		t.Fatalf("expected only visible repo to be indexed, got %#v", index.Repos)
	}
	if index.Repos[0].CheckoutPath != visibleRepo {
		t.Fatalf("unexpected indexed repo: %#v", index.Repos[0])
	}
}

func TestResolvePrefersRequestedTrustClassAndErrorsOnAmbiguity(t *testing.T) {
	t.Parallel()

	index := RepoIndex{
		Repos: []Repo{
			{Name: "shared", LocalName: "shared", CheckoutPath: "/trusted/shared", TrustClass: TrustClassTrusted},
			{Name: "shared", LocalName: "shared", CheckoutPath: "/external/shared", TrustClass: TrustClassExternal},
			{Name: "shared", LocalName: "shared", CheckoutPath: "/external/shared-2", TrustClass: TrustClassExternal},
		},
	}

	repo, err := index.Resolve("shared", TrustClassTrusted)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if repo.TrustClass != TrustClassTrusted {
		t.Fatalf("expected trusted preference, got %v", repo.TrustClass)
	}

	_, err = index.Resolve("shared", TrustClassExternal)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestResolveSupportsLocalFolderAlias(t *testing.T) {
	t.Parallel()

	index := RepoIndex{
		Repos: []Repo{
			{
				LocalName:    "math-app",
				Name:         "math",
				Slug:         "mikhail-yaskou/math",
				CheckoutPath: "/trusted/math-app",
				TrustClass:   TrustClassTrusted,
			},
		},
	}

	repo, err := index.Resolve("math-app", TrustClassTrusted)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if repo.CheckoutPath != "/trusted/math-app" {
		t.Fatalf("unexpected repo from local alias: %#v", repo)
	}

	repo, err = index.Resolve("mikhail-yaskou/math", TrustClassTrusted)
	if err != nil {
		t.Fatalf("Resolve returned error for slug: %v", err)
	}
	if repo.CheckoutPath != "/trusted/math-app" {
		t.Fatalf("unexpected repo from slug: %#v", repo)
	}
}

func TestFindOrCloneRepoClonesIntoExpectedRoot(t *testing.T) {
	h := testutil.NewHarness(t)
	h.CreateGitHubRemote("acme", "service")

	cfg := Config{
		TrustedDir:        h.TrustedRoot,
		ExternalDir:       h.ExternalRoot,
		TrustedGitHubOrgs: []string{"acme"},
	}

	repo, err := findOrCloneRepo(cfg, Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}, RepoIndex{}, "acme/service", TrustClassTrusted)
	if err != nil {
		t.Fatalf("findOrCloneRepo returned error: %v", err)
	}

	expected := filepath.Join(h.TrustedRoot, "acme", "service")
	if repo.CheckoutPath != expected {
		t.Fatalf("unexpected clone path: %s", repo.CheckoutPath)
	}
	if repo.Slug != "acme/service" {
		t.Fatalf("expected canonical slug after clone, got %#v", repo)
	}
	if _, err := os.Stat(filepath.Join(expected, ".git")); err != nil {
		t.Fatalf("expected cloned repo on disk: %v", err)
	}
}

func TestFindOrCloneRepoRejectsTrustedClassificationForUntrustedCommand(t *testing.T) {
	h := testutil.NewHarness(t)
	h.CreateGitHubRemote("acme", "service")

	cfg := Config{
		TrustedDir:        h.TrustedRoot,
		ExternalDir:       h.ExternalRoot,
		TrustedGitHubOrgs: []string{"acme"},
	}

	_, err := findOrCloneRepo(cfg, Runner{Env: []string{"GIT_CONFIG_GLOBAL=" + h.GitConfig}}, RepoIndex{}, "acme/service", TrustClassExternal)
	if err == nil || !strings.Contains(err.Error(), "use summon") {
		t.Fatalf("expected trusted classification guard, got %v", err)
	}

	_, err = RepoIndex{}.Resolve("missing", TrustClassTrusted)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestParseGitHubSlugPrefersSSHPatternOverGenericSplit(t *testing.T) {
	t.Parallel()

	owner, repo, ok := parseGitHubSlug("git@github.com:mikhail-yaskou/assistant.git")
	if !ok {
		t.Fatal("expected ssh github origin to parse")
	}
	if owner != "mikhail-yaskou" || repo != "assistant" {
		t.Fatalf("unexpected parsed slug: %s/%s", owner, repo)
	}
}
